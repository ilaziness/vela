package ssh

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	hostMetricsTTL     = 3 * time.Second
	hostMetricsTimeout = 5 * time.Second
)

// hostMetricsScript probes Linux hosts via /proc and df. Missing fields are omitted.
// CPU sampling sleeps 200ms and reports CPU_SAMPLE_MS for LatencyMs correction.
const hostMetricsScript = `export LC_ALL=C
printf 'HOSTNAME=%s\n' "$(hostname 2>/dev/null)"
if [ -r /proc/stat ]; then
  set -- $(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9, $5}' /proc/stat 2>/dev/null)
  t1=$1; i1=$2
  sleep 0.2
  echo CPU_SAMPLE_MS=200
  set -- $(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9, $5}' /proc/stat 2>/dev/null)
  t2=$1; i2=$2
  dt=$((t2-t1)); di=$((i2-i1))
  if [ -n "$dt" ] && [ "$dt" -gt 0 ] 2>/dev/null; then
    echo CPU_PCT=$(( (dt-di)*100/dt ))
  fi
fi
if [ -r /proc/meminfo ]; then
  awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{if(t>0){print "MEM_TOTAL_KB="t; print "MEM_AVAILABLE_KB="a}}' /proc/meminfo 2>/dev/null
fi
df -P -k / 2>/dev/null | awk 'NR==2 && $2+0>0 {print "DISK_TOTAL_KB="$2; print "DISK_USED_KB="$3}'
if [ -r /proc/loadavg ]; then
  awk '{print "LOAD1="$1}' /proc/loadavg 2>/dev/null
fi
exit 0`

// HostResources is a point-in-time remote resource snapshot for the UI.
type HostResources struct {
	Hostname   string  `json:"hostname,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemUsed    int64   `json:"mem_used,omitempty"`
	MemTotal   int64   `json:"mem_total,omitempty"`
	DiskUsed   int64   `json:"disk_used,omitempty"`
	DiskTotal  int64   `json:"disk_total,omitempty"`
	Load1      float64 `json:"load1,omitempty"`
	LatencyMs  int64   `json:"latency_ms,omitempty"`
	Supported  bool    `json:"supported"`
	HasCPU     bool    `json:"has_cpu"`
	HasMem     bool    `json:"has_mem"`
	HasDisk    bool    `json:"has_disk"`
	HasLoad    bool    `json:"has_load"`
}

type hostMetricsCacheEntry struct {
	res       HostResources
	fetchedAt time.Time
}

func (m *Manager) hostMetricsFetchMu(key string) *sync.Mutex {
	mu, _ := m.hostMetricsFetchLocks.LoadOrStore(key, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// FetchHostResources runs the host metrics probe for the session's SSH client.
// Results are deduplicated by clientKey with a short TTL.
func (m *Manager) FetchHostResources(ctx context.Context, linkID string) (HostResources, error) {
	if linkID == "" {
		return HostResources{}, fmt.Errorf("link id is required")
	}
	sess, err := m.GetSession(linkID)
	if err != nil {
		return HostResources{}, err
	}
	key := sess.ClientKey
	if key == "" {
		key = linkID
	}

	if cached, ok := m.loadHostMetricsFresh(key); ok {
		return cached, nil
	}

	mu := m.hostMetricsFetchMu(key)
	mu.Lock()
	defer mu.Unlock()

	if cached, ok := m.loadHostMetricsFresh(key); ok {
		return cached, nil
	}

	start := time.Now()
	output, err := m.Exec(ctx, linkID, hostMetricsScript, hostMetricsTimeout, DefaultExecMaxOut)
	elapsed := time.Since(start)
	if err != nil && strings.TrimSpace(output) == "" {
		m.logger.Debug("fetch host resources failed", zap.String("linkID", linkID), zap.Error(err))
		return HostResources{}, err
	}

	res, sampleMs := parseHostResourcesOutput(output)
	rtt := elapsed
	if sampleMs > 0 {
		rtt -= time.Duration(sampleMs) * time.Millisecond
		if rtt < 0 {
			rtt = 0
		}
	}
	res.LatencyMs = rtt.Milliseconds()
	m.hostMetricsCache.Store(key, &hostMetricsCacheEntry{res: res, fetchedAt: time.Now()})
	return res, nil
}

func (m *Manager) loadHostMetricsFresh(key string) (HostResources, bool) {
	v, ok := m.hostMetricsCache.Load(key)
	if !ok {
		return HostResources{}, false
	}
	entry := v.(*hostMetricsCacheEntry)
	if time.Since(entry.fetchedAt) >= hostMetricsTTL {
		return HostResources{}, false
	}
	return entry.res, true
}

// ParseHostResources parses KEY=value lines from hostMetricsScript output.
func ParseHostResources(output string) HostResources {
	res, _ := parseHostResourcesOutput(output)
	return res
}

func parseHostResourcesOutput(output string) (HostResources, int64) {
	var (
		res            HostResources
		sampleMs       int64
		memTotalKB     int64
		memAvailableKB int64
		diskTotalKB    int64
		diskUsedKB     int64
		hasMemTotal    bool
		hasMemAvail    bool
	)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(val, `"'`)
		switch key {
		case "HOSTNAME":
			res.Hostname = val
		case "CPU_SAMPLE_MS":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				sampleMs = n
			}
		case "CPU_PCT":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				if f < 0 {
					f = 0
				}
				if f > 100 {
					f = 100
				}
				res.CPUPercent = f
				res.HasCPU = true
			}
		case "MEM_TOTAL_KB":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				memTotalKB = n
				hasMemTotal = true
			}
		case "MEM_AVAILABLE_KB":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
				memAvailableKB = n
				hasMemAvail = true
			}
		case "DISK_TOTAL_KB":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				diskTotalKB = n
			}
		case "DISK_USED_KB":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
				diskUsedKB = n
			}
		case "LOAD1":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				res.Load1 = f
				res.HasLoad = true
			}
		}
	}

	if hasMemTotal && hasMemAvail {
		res.MemTotal = memTotalKB * 1024
		used := memTotalKB - memAvailableKB
		if used < 0 {
			used = 0
		}
		res.MemUsed = used * 1024
		res.HasMem = true
	}
	if diskTotalKB > 0 {
		res.DiskTotal = diskTotalKB * 1024
		if diskUsedKB > diskTotalKB {
			diskUsedKB = diskTotalKB
		}
		res.DiskUsed = diskUsedKB * 1024
		res.HasDisk = true
	}

	res.Supported = res.HasCPU || res.HasMem || res.HasDisk || res.HasLoad
	return res, sampleMs
}
