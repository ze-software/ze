package gnmi

import (
	"context"
	"slices"
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

	// gNMI specification Section 2.3.1: "Both the client and target MUST
	// support the `JSON` encoding as a minimum."
	if !slices.Equal(resp.SupportedEncodings, []gpb.Encoding{gpb.Encoding_JSON, gpb.Encoding_JSON_IETF}) {
		t.Errorf("supported encodings = %v, want [JSON JSON_IETF]", resp.SupportedEncodings)
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
