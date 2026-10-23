package ssh

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	hostMetricsTTL = 3 * time.Second
	// Allow slow overseas RTT: remote probe ~0.2s+ plus one Exec round-trip.
	hostMetricsTimeout = 10 * time.Second
	// Fallback when /proc/uptime is unavailable or delta is nonsense.
	hostMetricsSampleFallbackMs = 200
)

//go:embed hostmetrics.sh
var hostMetricsScriptFile string

// hostMetricsScript is the probe body sent via SSH Exec (LF-normalized).
var hostMetricsScript string

func init() {
	// Windows checkouts may embed CRLF; remote /bin/sh rejects CR in scripts.
	hostMetricsScript = strings.ReplaceAll(hostMetricsScriptFile, "\r\n", "\n")
	hostMetricsScript = strings.ReplaceAll(hostMetricsScript, "\r", "\n")
}

// NetIfaceStats is per-NIC receive/transmit throughput in bytes per second.
type NetIfaceStats struct {
	Name  string `json:"name"`
	RxBps int64  `json:"rx_bps"`
	TxBps int64  `json:"tx_bps"`
}

// DiskIOStats is per-disk read/write throughput in bytes per second.
type DiskIOStats struct {
	Name     string `json:"name"`
	ReadBps  int64  `json:"read_bps"`
	WriteBps int64  `json:"write_bps"`
}

// HostResources is a point-in-time remote resource snapshot for the UI.
type HostResources struct {
	Hostname   string          `json:"hostname,omitempty"`
	CPUPercent float64         `json:"cpu_percent,omitempty"`
	MemUsed    int64           `json:"mem_used,omitempty"`
	MemTotal   int64           `json:"mem_total,omitempty"`
	DiskUsed   int64           `json:"disk_used,omitempty"`
	DiskTotal  int64           `json:"disk_total,omitempty"`
	Load1      float64         `json:"load1,omitempty"`
	LatencyMs  int64           `json:"latency_ms,omitempty"`
	NetIfaces  []NetIfaceStats `json:"net_ifaces,omitempty"`
	DiskIO     []DiskIOStats   `json:"disk_io,omitempty"`
	Supported  bool            `json:"supported"`
	HasCPU     bool            `json:"has_cpu"`
	HasMem     bool            `json:"has_mem"`
	HasDisk    bool            `json:"has_disk"`
	HasLoad    bool            `json:"has_load"`
	HasNet     bool            `json:"has_net"`
	HasDiskIO  bool            `json:"has_disk_io"`
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

func (m *Manager) clearHostMetrics(clientKey string) {
	if clientKey == "" {
		return
	}
	m.hostMetricsCache.Delete(clientKey)
	m.hostMetricsFetchLocks.Delete(clientKey)
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
		netByName      = map[string]NetIfaceStats{}
		diskIOByName   = map[string]DiskIOStats{}
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
		case "SAMPLE_MS":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				sampleMs = n
			}
		case "CPU_SAMPLE_MS":
			// Legacy alias; ignore when SAMPLE_MS already set (either order in output).
			if sampleMs > 0 {
				break
			}
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
		case "NET":
			if name, a, b, ok := parseNameTwoRates(val); ok {
				netByName[name] = NetIfaceStats{Name: name, RxBps: a, TxBps: b}
			}
		case "DISKIO":
			if name, a, b, ok := parseNameTwoRates(val); ok {
				diskIOByName[name] = DiskIOStats{Name: name, ReadBps: a, WriteBps: b}
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
	if len(netByName) > 0 {
		res.NetIfaces = valuesBySortedKeys(netByName)
		res.HasNet = true
	}
	if len(diskIOByName) > 0 {
		res.DiskIO = valuesBySortedKeys(diskIOByName)
		res.HasDiskIO = true
	}

	// Only fall back when a timed probe likely ran (CPU/net/disk-io rates).
	if sampleMs <= 0 && (res.HasCPU || res.HasNet || res.HasDiskIO) {
		sampleMs = hostMetricsSampleFallbackMs
	}

	res.Supported = res.HasCPU || res.HasMem || res.HasDisk || res.HasLoad || res.HasNet || res.HasDiskIO
	return res, sampleMs
}

func valuesBySortedKeys[T any](m map[string]T) []T {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]T, 0, len(names))
	for _, name := range names {
		out = append(out, m[name])
	}
	return out
}

// parseNameTwoRates parses "name:a:b" into name and two non-negative int64 rates.
func parseNameTwoRates(val string) (name string, a, b int64, ok bool) {
	name, rest, ok := strings.Cut(val, ":")
	if !ok || name == "" {
		return "", 0, 0, false
	}
	aStr, bStr, ok := strings.Cut(rest, ":")
	if !ok {
		return "", 0, 0, false
	}
	a, err1 := strconv.ParseInt(aStr, 10, 64)
	b, err2 := strconv.ParseInt(bStr, 10, 64)
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	return name, a, b, true
}
