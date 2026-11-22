// Design: docs/architecture/api/process-protocol.md -- selected initial readiness.
// Related: rib_selected_dispatch.go -- final receipts precede readiness.
package rib

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func (r *RIBManager) signalSelectedReady(ctx context.Context, peer string, session uint64) error {
	var command textbuf.Buffer
	text := command.Str("request peer ").Str(peer).Str(" plugin session ready session ").Uint(session).String()
	status, _, err := r.plugin.DispatchCommand(ctx, text)
	if err != nil {
		return fmt.Errorf("selected initial output to %s Session %d could not report readiness: %w; inspect the destination session before retrying", peer, session, err)
	}
	if status == statusError {
		return fmt.Errorf("selected initial output to %s Session %d was refused readiness; inspect the destination session before retrying", peer, session)
	}
	return nil
}

// SDK Run MUST drain event application before this returns. Its workers join
// before the source lease owner, Loc-RIB and recovery snapshots are released.
func (r *RIBManager) stopSelectedDispatch() {
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	err := r.selectedDispatch.stop(ctx)
	cancel()
	if err == nil {
		return
	}
	logger().Warn("BGP selected destination workers did not retire within their shutdown budget", "error", err,
		"code", codeDoctorBGPSelectedOutput, "fix", "ze request restart")
	// Stop has no refusal channel. Its canceled workers must finish before
	// their shared capture/source owners can be released by later defers.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if err := r.selectedDispatch.stop(ctx); err != nil {
		panic("BUG: noncancelable selected shutdown failed: " + err.Error())
	}
}
