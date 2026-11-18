// Design: docs/architecture/api/process-protocol.md -- exact-connection snapshot reads.
// Related: current_selection.go, origin_admission_lookup.go -- typed cutover stays at each caller.
package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type readSnapshotInput interface {
	Validate(time.Time) error
}

// callReadSnapshot shares only raw framed reads. The caller MUST choose the
// published typed bridge or Mux first; this helper MUST NOT choose a transport
// or retry a refused bridge through the socket. Each contract supplies its
// deadline and exact output validator. No connection or output is retained.
func callReadSnapshot[Input readSnapshotInput, Output any](ctx context.Context, conn *rpc.Conn, input Input,
	callContext func(context.Context, *Input) (context.Context, context.CancelFunc),
	method string, validate func(*Output, Input) error) (*Output, error) {
	ctx, cancel := callContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	data, err := conn.CallRPC(ctx, method, input)
	if err != nil {
		return nil, err
	}
	var output Output
	if err := json.Unmarshal(data, &output); err != nil {
		// Both registered callback names carry this namespace separator.
		_, name, _ := strings.Cut(method, ":")
		return nil, fmt.Errorf("%s reply: %w", name, err)
	}
	if err := validate(&output, input); err != nil {
		return nil, err
	}
	return &output, nil
}
