// Design: docs/architecture/api/architecture.md -- gNMI Capabilities RPC
// Related: server.go -- gNMI server core
// gNMI Specification Section 3.2: Capabilities RPC

package gnmi

import (
	"context"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// Capabilities returns the set of YANG models and encodings supported by Ze.
func (s *Server) Capabilities(_ context.Context, _ *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	if s.metrics != nil {
		s.metrics.requestsTotal.With("Capabilities").Inc()
	}
	s.modelsOnce.Do(s.loadModels)

	return &gpb.CapabilityResponse{
		SupportedModels:    s.models,
		SupportedEncodings: []gpb.Encoding{gpb.Encoding_JSON_IETF},
		GNMIVersion:        "0.10.0",
	}, nil
}

func (s *Server) loadModels() {
	if s.loader == nil {
		return
	}
	loader, err := s.loader()
	if err != nil {
		logger.Warn("gnmi: load YANG modules", "error", err)
		return
	}
	if loader == nil {
		return
	}
	for _, name := range loader.ModuleNames() {
		mod := loader.GetModule(name)
		if mod == nil {
			continue
		}
		md := &gpb.ModelData{Name: mod.Name}
		if mod.Organization != nil {
			md.Organization = mod.Organization.Name
		}
		if len(mod.Revision) > 0 {
			md.Version = mod.Revision[0].Name
		}
		s.models = append(s.models, md)
	}
}
