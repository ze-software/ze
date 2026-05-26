package host

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
		got := nvmeNamespace(tt.input)
		if got != tt.want {
			t.Errorf("nvmeNamespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseNVMeSMARTBuf_Healthy(t *testing.T) {
	var buf [512]byte
	// Critical Warning = 0 (healthy)
	buf[0] = 0
	// Temperature = 313K (40C) little-endian
	binary.LittleEndian.PutUint16(buf[1:3], 313)
	// Power On Hours = 12345
	binary.LittleEndian.PutUint64(buf[128:136], 12345)
	// Error count = 2
	binary.LittleEndian.PutUint64(buf[176:184], 2)

	info := parseNVMeSMARTBuf(&buf)
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

func TestParseNVMeSMARTBuf_CriticalWarning(t *testing.T) {
	var buf [512]byte
	buf[0] = 0x04
	binary.LittleEndian.PutUint16(buf[1:3], 350)

	info := parseNVMeSMARTBuf(&buf)
	if info.Healthy {
		t.Error("expected unhealthy with critical warning")
	}
	if info.TempCelsius != 77 {
		t.Errorf("TempCelsius = %d, want 77 (350K - 273)", info.TempCelsius)
	}
}

func TestParseNVMeSMARTBuf_ZeroKelvin(t *testing.T) {
	var buf [512]byte
	// Temperature = 0K means "not reported"
	binary.LittleEndian.PutUint16(buf[1:3], 0)

	info := parseNVMeSMARTBuf(&buf)
	if info.TempCelsius != 0 {
		t.Errorf("TempCelsius = %d, want 0 (not reported)", info.TempCelsius)
	}
}
