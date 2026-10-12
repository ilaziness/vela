package ssh

import "testing"

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

func TestParseHostResourcesCPUSampleMs(t *testing.T) {
	res, sampleMs := parseHostResourcesOutput("CPU_SAMPLE_MS=200\nCPU_PCT=10\n")
	if sampleMs != 200 || !res.HasCPU {
		t.Fatalf("got res=%+v sampleMs=%d", res, sampleMs)
	}
}
