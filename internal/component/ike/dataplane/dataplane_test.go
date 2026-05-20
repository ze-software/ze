package dataplane

import (
	"net"
	"testing"
)

type mockDataplane struct {
	installed   []SAParams
	removed     []uint32
	policies    []SPParams
	removedPols int
}

func (m *mockDataplane) InstallSA(p SAParams) error {
	m.installed = append(m.installed, p)
	return nil
}

func (m *mockDataplane) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	m.removed = append(m.removed, spi)
	return nil
}

func (m *mockDataplane) InstallPolicy(p SPParams) error {
	m.policies = append(m.policies, p)
	return nil
}

func (m *mockDataplane) RemovePolicy(_, _ *net.IPNet, _ SADir) error {
	m.removedPols++
	return nil
}

func (m *mockDataplane) ListSAs(_ uint32) ([]SAInfo, error) {
	return nil, nil
}

func (m *mockDataplane) Close() error { return nil }

func TestDataplaneInterface(t *testing.T) {
	var dp Dataplane = &mockDataplane{}

	err := dp.InstallSA(SAParams{
		SPI:   0x12345678,
		Src:   net.ParseIP("10.0.0.1"),
		Dst:   net.ParseIP("10.0.0.2"),
		IfID:  1,
		Proto: 50,
		Mode:  2,
	})
	if err != nil {
		t.Fatalf("InstallSA: %v", err)
	}

	err = dp.InstallPolicy(SPParams{
		Src:   &net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)},
		Dst:   &net.IPNet{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)},
		Dir:   SADirOut,
		Proto: 50,
		Mode:  2,
		IfID:  1,
	})
	if err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}

	if err = dp.RemoveSA(0x12345678, net.ParseIP("10.0.0.2"), 50); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}

	m, ok := dp.(*mockDataplane)
	if !ok {
		t.Fatal("unexpected dataplane type")
	}
	if len(m.installed) != 1 {
		t.Errorf("installed count = %d, want 1", len(m.installed))
	}
	if m.installed[0].SPI != 0x12345678 {
		t.Errorf("installed SPI = %d, want 0x12345678", m.installed[0].SPI)
	}
	if len(m.removed) != 1 || m.removed[0] != 0x12345678 {
		t.Errorf("removed = %v, want [0x12345678]", m.removed)
	}
}

func TestRegisterAndLoad(t *testing.T) {
	mu.Lock()
	saved := backends
	backends = make(map[string]func() (Dataplane, error))
	active = nil
	mu.Unlock()

	defer func() {
		mu.Lock()
		backends = saved
		active = nil
		mu.Unlock()
	}()

	err := Register("test", func() (Dataplane, error) {
		return &mockDataplane{}, nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	err = Register("test", func() (Dataplane, error) {
		return &mockDataplane{}, nil
	})
	if err == nil {
		t.Fatal("duplicate Register should fail")
	}

	if err := Load("test"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	dp := Get()
	if dp == nil {
		t.Fatal("Get returned nil after Load")
	}

	if err := Load("nonexistent"); err == nil {
		t.Fatal("Load nonexistent should fail")
	}

	if err := CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if Get() != nil {
		t.Fatal("Get should return nil after Close")
	}
}

func TestSADirValues(t *testing.T) {
	if SADirIn != 1 {
		t.Errorf("SADirIn = %d, want 1", SADirIn)
	}
	if SADirOut != 2 {
		t.Errorf("SADirOut = %d, want 2", SADirOut)
	}
	if SADirFwd != 3 {
		t.Errorf("SADirFwd = %d, want 3", SADirFwd)
	}
}
