package ssh

import (
	"strings"
	"testing"
)

func TestHostMetricsScriptEmbedded(t *testing.T) {
	if strings.TrimSpace(hostMetricsScript) == "" {
		t.Fatal("embedded hostmetrics.sh is empty")
	}
	if strings.Contains(hostMetricsScript, "\r") {
		t.Fatal("embedded hostmetrics.sh still contains CR after normalize")
	}
	for _, needle := range []string{
		"SAMPLE_MS=",
		"CPU_PCT=",
		"NET=",
		"DISKIO=",
		"/proc/diskstats",
		"/proc/net/dev",
	} {
		if !strings.Contains(hostMetricsScript, needle) {
			t.Fatalf("embedded script missing %q", needle)
		}
	}
}

func TestParseHostResources(t *testing.T) {
	output := `HOSTNAME=box1
CPU_PCT=37
MEM_TOTAL_KB=8042348
MEM_AVAILABLE_KB=2048000
DISK_TOTAL_KB=104857600
DISK_USED_KB=52428800
LOAD1=1.25
`
	res := ParseHostResources(output)
	if !res.Supported {
		t.Fatal("expected supported")
	}
	if res.Hostname != "box1" {
		t.Errorf("hostname: got %q", res.Hostname)
	}
	if !res.HasCPU || res.CPUPercent != 37 {
		t.Errorf("cpu: has=%v pct=%v", res.HasCPU, res.CPUPercent)
	}
	if !res.HasMem || res.MemTotal != 8042348*1024 {
		t.Errorf("mem total: has=%v total=%d", res.HasMem, res.MemTotal)
	}
	wantUsed := int64(8042348-2048000) * 1024
	if res.MemUsed != wantUsed {
		t.Errorf("mem used: got %d want %d", res.MemUsed, wantUsed)
	}
	if !res.HasDisk || res.DiskTotal != 104857600*1024 || res.DiskUsed != 52428800*1024 {
		t.Errorf("disk: has=%v used=%d total=%d", res.HasDisk, res.DiskUsed, res.DiskTotal)
	}
	if !res.HasLoad || res.Load1 != 1.25 {
		t.Errorf("load: has=%v load1=%v", res.HasLoad, res.Load1)
	}
}

func TestParseHostResourcesEmpty(t *testing.T) {
	res := ParseHostResources("")
	if res.Supported {
		t.Fatal("expected unsupported for empty output")
	}
}

func TestParseHostResourcesPartial(t *testing.T) {
	res := ParseHostResources("LOAD1=0.5\nHOSTNAME=onlyload\n")
	if !res.Supported || !res.HasLoad || res.HasCPU || res.HasMem || res.HasDisk {
		t.Fatalf("unexpected flags: %+v", res)
	}
	if res.Hostname != "onlyload" || res.Load1 != 0.5 {
		t.Fatalf("got %+v", res)
	}
}

func TestParseHostResourcesClampsCPU(t *testing.T) {
	res := ParseHostResources("CPU_PCT=150\n")
	if !res.HasCPU || res.CPUPercent != 100 {
		t.Fatalf("got cpu=%v has=%v", res.CPUPercent, res.HasCPU)
	}
}

func TestParseHostResourcesNegativeCPU(t *testing.T) {
	res := ParseHostResources("CPU_PCT=-5\n")
	if !res.HasCPU || res.CPUPercent != 0 {
		t.Fatalf("got cpu=%v has=%v", res.CPUPercent, res.HasCPU)
	}
}

func TestParseHostResourcesMemWithoutAvailable(t *testing.T) {
	res := ParseHostResources("MEM_TOTAL_KB=1024\n")
	if res.HasMem || res.Supported {
		t.Fatalf("expected no mem without available: %+v", res)
	}
}

func TestParseHostResourcesSampleMs(t *testing.T) {
	res, sampleMs := parseHostResourcesOutput("SAMPLE_MS=240\nCPU_PCT=10\n")
	if sampleMs != 240 || !res.HasCPU {
		t.Fatalf("got res=%+v sampleMs=%d", res, sampleMs)
	}
}

func TestParseHostResourcesCPUSampleMsAlias(t *testing.T) {
	res, sampleMs := parseHostResourcesOutput("CPU_SAMPLE_MS=200\nCPU_PCT=10\n")
	if sampleMs != 200 || !res.HasCPU {
		t.Fatalf("got res=%+v sampleMs=%d", res, sampleMs)
	}
}

func TestParseHostResourcesSampleMsPreferred(t *testing.T) {
	_, sampleMs := parseHostResourcesOutput("CPU_SAMPLE_MS=200\nSAMPLE_MS=240\nCPU_PCT=10\n")
	if sampleMs != 240 {
		t.Fatalf("SAMPLE_MS should win, got %d", sampleMs)
	}
	_, sampleMs = parseHostResourcesOutput("SAMPLE_MS=240\nCPU_SAMPLE_MS=200\nCPU_PCT=10\n")
	if sampleMs != 240 {
		t.Fatalf("SAMPLE_MS should win when first, got %d", sampleMs)
	}
}

func TestParseHostResourcesSampleMsFallback(t *testing.T) {
	_, sampleMs := parseHostResourcesOutput("CPU_PCT=10\n")
	if sampleMs != hostMetricsSampleFallbackMs {
		t.Fatalf("want fallback %d, got %d", hostMetricsSampleFallbackMs, sampleMs)
	}
	_, sampleMs = parseHostResourcesOutput("")
	if sampleMs != 0 {
		t.Fatalf("empty output should not invent sample ms, got %d", sampleMs)
	}
}

func TestParseHostResourcesNetIfaces(t *testing.T) {
	res := ParseHostResources(`NET=eth0:1024:2048
NET=wlan0:0:512
NET=bad
NET=:1:2
`)
	if !res.HasNet || !res.Supported {
		t.Fatalf("expected has_net: %+v", res)
	}
	if len(res.NetIfaces) != 2 {
		t.Fatalf("want 2 ifaces, got %+v", res.NetIfaces)
	}
	// Sorted by name.
	if res.NetIfaces[0].Name != "eth0" || res.NetIfaces[0].RxBps != 1024 || res.NetIfaces[0].TxBps != 2048 {
		t.Fatalf("eth0: %+v", res.NetIfaces[0])
	}
	if res.NetIfaces[1].Name != "wlan0" || res.NetIfaces[1].RxBps != 0 || res.NetIfaces[1].TxBps != 512 {
		t.Fatalf("wlan0: %+v", res.NetIfaces[1])
	}
}

func TestParseHostResourcesNetNegativeClamped(t *testing.T) {
	res := ParseHostResources("NET=eth0:-10:-20\n")
	if !res.HasNet || len(res.NetIfaces) != 1 {
		t.Fatalf("got %+v", res)
	}
	if res.NetIfaces[0].RxBps != 0 || res.NetIfaces[0].TxBps != 0 {
		t.Fatalf("expected clamp to 0: %+v", res.NetIfaces[0])
	}
}

func TestParseHostResourcesDiskIO(t *testing.T) {
	res := ParseHostResources(`DISKIO=nvme0n1:4096:8192
DISKIO=sda:0:512
DISKIO=bad
`)
	if !res.HasDiskIO || !res.Supported {
		t.Fatalf("expected has_disk_io: %+v", res)
	}
	if len(res.DiskIO) != 2 {
		t.Fatalf("want 2 disks, got %+v", res.DiskIO)
	}
	if res.DiskIO[0].Name != "nvme0n1" || res.DiskIO[0].ReadBps != 4096 || res.DiskIO[0].WriteBps != 8192 {
		t.Fatalf("nvme0n1: %+v", res.DiskIO[0])
	}
	if res.DiskIO[1].Name != "sda" || res.DiskIO[1].ReadBps != 0 || res.DiskIO[1].WriteBps != 512 {
		t.Fatalf("sda: %+v", res.DiskIO[1])
	}
}
