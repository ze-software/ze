// Design: docs/architecture/api/architecture.md -- gNMI Capabilities RPC
// Related: server.go -- gNMI server core
// gNMI Specification Section 3.2: Capabilities RPC

package gnmi

import (
	"context"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Capabilities returns the set of YANG models and encodings supported by Ze.
func (s *Server) Capabilities(_ context.Context, _ *gpb.CapabilityRequest) (*gpb.CapabilityResponse, error) {
	if s.metrics != nil {
		s.metrics.requestsTotal.With("Capabilities").Inc()
	}
	s.modelsOnce.Do(s.loadModels)

	return &gpb.CapabilityResponse{
		SupportedModels:    s.models,
		SupportedEncodings: supportedEncodings,
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

// answerEncoding is the JSON encoding a request asked Ze to answer in. Ze
// serialises config one way, and the payload is valid JSON, so it serves both
// JSON encodings from the same bytes and only the TypedValue member differs.
//
// gNMI specification Section 2.3.1: "Data specified with a type of JSON MUST be
// valid JSON, but no additional constraints are placed upon it."
// gNMI specification Section 2.3.1: "Both the client and target MUST support the
// `JSON` encoding as a minimum."
type answerEncoding uint8

const (
	// answerEncodingUnspecified is the zero value, which no request selects:
	// answerEncodingFor answers JSON or JSON_IETF, or refuses.
	answerEncodingUnspecified answerEncoding = iota
	answerEncodingJSON
	answerEncodingJSONIETF
)

// supportedEncodings is what Capabilities advertises, the two encodings
// answerEncodingFor accepts, so the advertisement and the refusal cannot
// disagree.
var supportedEncodings = []gpb.Encoding{gpb.Encoding_JSON, gpb.Encoding_JSON_IETF}

// answerEncodingFor answers the encoding a Get or Subscribe asked for, and an
// Unimplemented status naming it for any encoding Ze cannot serialise. JSON is
// the proto3 zero, so a request that leaves the field unset is answered in
// JSON, which is what gnmic sends by default.
//
// gNMI specification Section 3.3.1: "If the encoding is not specified, JSON MUST
// be used. If the target does not support the specified encoding, the target
// MUST return an error of `Unimplemented`. The error message MUST indicate that
// the specified encoding is unsupported."
func answerEncodingFor(requested gpb.Encoding) (answerEncoding, error) {
	switch requested {
	case gpb.Encoding_JSON:
		return answerEncodingJSON, nil
	case gpb.Encoding_JSON_IETF:
		return answerEncodingJSONIETF, nil
	case gpb.Encoding_BYTES, gpb.Encoding_PROTO, gpb.Encoding_ASCII:
		return answerEncodingUnspecified, unsupportedEncoding(requested)
	default:
		// The set is open: a client's enum may carry a number this build does not name.
		return answerEncodingUnspecified, unsupportedEncoding(requested)
	}
}

// unsupportedEncoding is the refusal answerEncodingFor gives for requested.
func unsupportedEncoding(requested gpb.Encoding) error {
	return status.Errorf(codes.Unimplemented,
		"encoding %s is not supported: Ze serialises config as JSON only, so set the request encoding to JSON or JSON_IETF (gnmic --encoding json_ietf)",
		requested)
}

// typedValue wraps data, a JSON serialisation of config, in the TypedValue
// member of the encoding the request asked for.
func (e answerEncoding) typedValue(data []byte) *gpb.TypedValue {
	switch e {
	case answerEncodingJSON:
		return &gpb.TypedValue{Value: &gpb.TypedValue_JsonVal{JsonVal: data}}
	case answerEncodingJSONIETF:
		return &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: data}}
	case answerEncodingUnspecified:
	}
	panic("BUG: gnmi value encoded with no answer encoding: answerEncodingFor refuses before any value is encoded")
}

// notification answers n in this encoding. The change notifier builds every
// notification once in JSON_IETF and shares it between subscribers, so a JSON
// subscriber gets a clone whose JSON_IETF values are rewrapped as JSON, and n
// itself is never changed. A notification is a config change, never a wire
// path, so the clone's copy is affordable.
//
// gNMI specification Section 2.3.1: "An implementation MUST NOT serialise data
// with mixed `JSON` and `JSON_IETF` encodings."
func (e answerEncoding) notification(n *gpb.Notification) *gpb.Notification {
	if e == answerEncodingJSONIETF {
		return n
	}
	answer, isNotification := proto.Clone(n).(*gpb.Notification)
	if !isNotification {
		panic("BUG: proto.Clone of a gnmi Notification answered another message type")
	}
	for _, update := range answer.GetUpdate() {
		ietf, isIETF := update.GetVal().GetValue().(*gpb.TypedValue_JsonIetfVal)
		if !isIETF {
			continue
		}
		update.Val = e.typedValue(ietf.JsonIetfVal)
	}
	return answer
}
