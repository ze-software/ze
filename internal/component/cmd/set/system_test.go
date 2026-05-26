package set

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestHandleSetSystemFD_NoArgs(t *testing.T) {
	resp, err := handleSetSystemFD(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("status = %v, want error", resp.Status)
	}
}

func TestHandleSetSystemFD_InvalidArg(t *testing.T) {
	resp, err := handleSetSystemFD(nil, []string{"abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("status = %v, want error", resp.Status)
	}
}

func TestHandleSetSystemFD_Zero(t *testing.T) {
	resp, err := handleSetSystemFD(nil, []string{"0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("status = %v, want error for zero limit", resp.Status)
	}
}
