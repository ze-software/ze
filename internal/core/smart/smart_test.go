// Design: docs/architecture/storage/smart-health.md -- SMART disk health management

package smart

import (
	"encoding/binary"
	"testing"
)

func TestNvmeNamespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"nvme0n1", "nvme0n1"},
		{"nvme0n1p1", "nvme0n1"},
		{"nvme0n1p12", "nvme0n1"},
		{"nvme0n1p0", "nvme0n1"},
		{"nvme1n2p3", "nvme1n2"},
		{"nvme0n1p", "nvme0n1p"},
		{"sda", "sda"},
		{"sda1", "sda1"},
		{"nvme0", "nvme0"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NvmeNamespace(tt.input)
		if got != tt.want {
			t.Errorf("NvmeNamespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseNVMeBuf_Healthy(t *testing.T) {
	var buf [512]byte
	buf[0] = 0
	binary.LittleEndian.PutUint16(buf[1:3], 313)
	binary.LittleEndian.PutUint64(buf[128:136], 12345)
	binary.LittleEndian.PutUint64(buf[176:184], 2)

	info := ParseNVMeBuf(&buf)
	if !info.Healthy {
		t.Error("expected healthy")
	}
	if info.TempCelsius != 40 {
		t.Errorf("TempCelsius = %d, want 40", info.TempCelsius)
	}
	if info.PowerOnHours != 12345 {
		t.Errorf("PowerOnHours = %d, want 12345", info.PowerOnHours)
	}
	if info.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", info.ErrorCount)
	}
}

func TestParseNVMeBuf_CriticalWarning(t *testing.T) {
	var buf [512]byte
	buf[0] = 0x04
	binary.LittleEndian.PutUint16(buf[1:3], 350)

	info := ParseNVMeBuf(&buf)
	if info.Healthy {
		t.Error("expected unhealthy with critical warning")
	}
	if info.TempCelsius != 77 {
		t.Errorf("TempCelsius = %d, want 77 (350K - 273)", info.TempCelsius)
	}
}

func TestParseNVMeBuf_ZeroKelvin(t *testing.T) {
	var buf [512]byte
	binary.LittleEndian.PutUint16(buf[1:3], 0)

	info := ParseNVMeBuf(&buf)
	if info.TempCelsius != 0 {
		t.Errorf("TempCelsius = %d, want 0 (not reported)", info.TempCelsius)
	}
}
