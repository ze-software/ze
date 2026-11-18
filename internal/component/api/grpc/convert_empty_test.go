// Design: docs/architecture/api/architecture.md -- lossless gRPC parameters.
package grpc

import (
	"testing"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
)

// TestGRPCCommandsPreserveExplicitEmptyContent covers both protobuf request
// adapters so an explicit empty value does not become an omitted argument.
func TestGRPCCommandsPreserveExplicitEmptyContent(t *testing.T) {
	input := &zepb.CommandRequest{Command: "show data encode", Params: map[string]string{"content": ""}}
	execute, err := fromProtoExecuteRequest(input, api.CallerIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := fromProtoStreamRequest(input, api.CallerIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{execute.Command, stream.Command} {
		if want := `show data encode content ""`; got != want {
			t.Fatalf("command = %q, want %q", got, want)
		}
	}
}
