package gnmi

import (
	"context"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	yangloader "github.com/ze-software/ze/internal/component/config/yang"
)

func TestCapabilitiesResponse(t *testing.T) {
	srv := NewServer(Config{}, nil, nil, yangloader.DefaultLoader, nil)

	resp, err := srv.Capabilities(context.Background(), &gpb.CapabilityRequest{})
	if err != nil {
		t.Fatalf("Capabilities() error: %v", err)
	}

	if len(resp.SupportedModels) == 0 {
		t.Error("expected at least one supported model")
	}

	if len(resp.SupportedEncodings) == 0 {
		t.Fatal("expected at least one encoding")
	}
	if resp.SupportedEncodings[0] != gpb.Encoding_JSON_IETF {
		t.Errorf("expected JSON_IETF encoding, got %v", resp.SupportedEncodings[0])
	}

	if resp.GNMIVersion == "" {
		t.Error("expected non-empty gNMI version")
	}
}

func TestCapabilitiesNoLoader(t *testing.T) {
	srv := NewServer(Config{}, nil, nil, nil, nil)

	resp, err := srv.Capabilities(context.Background(), &gpb.CapabilityRequest{})
	if err != nil {
		t.Fatalf("Capabilities() error: %v", err)
	}
	if len(resp.SupportedModels) != 0 {
		t.Errorf("expected no models without loader, got %d", len(resp.SupportedModels))
	}
}
