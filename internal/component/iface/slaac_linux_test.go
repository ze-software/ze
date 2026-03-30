package iface

import "testing"

func TestEnableSLAAC(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	if err := EnableSLAAC("eth0"); err != nil {
		t.Fatalf("EnableSLAAC: %v", err)
	}
	got := readSysctl(t, root, "net/ipv6/conf/eth0/autoconf")
	if got != "1" {
		t.Errorf("got %q, want %q", got, "1")
	}
}

func TestDisableSLAAC(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	if err := DisableSLAAC("eth0"); err != nil {
		t.Fatalf("DisableSLAAC: %v", err)
	}
	got := readSysctl(t, root, "net/ipv6/conf/eth0/autoconf")
	if got != "0" {
		t.Errorf("got %q, want %q", got, "0")
	}
}

func TestSLAACInvalidName(t *testing.T) {
	if err := EnableSLAAC(""); err == nil {
		t.Error("expected error for empty name")
	}
	if err := DisableSLAAC(""); err == nil {
		t.Error("expected error for empty name")
	}
}
