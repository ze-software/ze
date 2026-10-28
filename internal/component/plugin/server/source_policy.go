// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle.
package server

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SourcePolicyOwner captures one immutable process and connection identity.
// A coordinator MUST retain these handles through prepare/apply/abort/finalize,
// never recapture successors to complete work owned by an earlier lifetime.
// Safe for concurrent calls; transaction sequencing belongs to the coordinator.
type SourcePolicyOwner struct {
	name   string
	server *Server
	proc   *process.Process
	conn   *ipc.PluginConn
}

// CaptureSourcePolicyOwners preflights the selected processes' Stage 1 roles.
// Observers are excluded. A declared owner without a handler refuses the whole
// capture before any callback runs. Callers MUST capture before policy publication.
func (s *Server) CaptureSourcePolicyOwners(names []string) ([]*SourcePolicyOwner, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, fmt.Errorf("source-policy: plugin process manager is unavailable")
	}
	owners := make([]*SourcePolicyOwner, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		proc := pm.GetProcess(name)
		if proc == nil {
			return nil, fmt.Errorf("source-policy owner %s: selected process is unavailable", name)
		}
		reg := proc.Registration()
		if reg == nil {
			return nil, fmt.Errorf("source-policy owner %s: Stage 1 registration is unavailable", name)
		}
		if !reg.SourcePolicyOwner {
			if reg.SourcePolicy {
				return nil, fmt.Errorf("source-policy owner %s: handler requires source-policy-owner declaration", name)
			}
			continue
		}
		if !reg.SourcePolicy {
			return nil, fmt.Errorf("source-policy owner %s retains source state but has no lifecycle handler; upgrade the plugin to implement source-policy before changing source policy", name)
		}
		owner := &SourcePolicyOwner{name: name, server: s, proc: proc, conn: proc.Conn()}
		if err := owner.current(); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, nil
}

// Name returns the selected process name, not its implementation alias.
func (owner *SourcePolicyOwner) Name() string { return owner.name }

// Call waits for actual application on the captured owner, never a successor.
// Before prepare the coordinator MUST settle its receive-publication frontier;
// this method then strictly drains event application. Callers MUST NOT hold any
// locks handlers need. Owners MUST settle asynchronous and downstream work before
// acknowledging; the process FIFO alone cannot establish that settlement.
func (owner *SourcePolicyOwner) Call(ctx context.Context, input rpc.SourcePolicyInput) error {
	if err := input.Validate(time.Now()); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.current(); err != nil {
		return err
	}
	if input.Operation == rpc.SourcePolicyPrepare {
		if err := owner.proc.DrainEventsApplied(ctx); err != nil {
			return err
		}
		if err := owner.current(); err != nil {
			return err
		}
	}
	if err := owner.conn.SendSourcePolicy(ctx, input); err != nil {
		return fmt.Errorf("source-policy owner %s: %w", owner.name, err)
	}
	if err := owner.current(); err != nil {
		return err
	}
	return ctx.Err()
}

func (owner *SourcePolicyOwner) current() error {
	if owner == nil {
		return fmt.Errorf("source-policy owner is uninitialized")
	}
	if owner.server == nil {
		return fmt.Errorf("source-policy owner is uninitialized")
	}
	pm := owner.server.procManager.Load()
	if pm == nil {
		return fmt.Errorf("source-policy owner %s: captured process lifetime ended", owner.name)
	}
	if owner.proc == nil {
		return fmt.Errorf("source-policy owner %s: captured process is missing", owner.name)
	}
	if pm.GetProcess(owner.name) != owner.proc {
		return fmt.Errorf("source-policy owner %s: captured process lifetime ended", owner.name)
	}
	if owner.conn == nil {
		return fmt.Errorf("source-policy owner %s: captured connection is missing", owner.name)
	}
	if owner.proc.Conn() != owner.conn {
		return fmt.Errorf("source-policy owner %s: captured connection lifetime ended", owner.name)
	}
	if owner.proc.Done() == nil {
		return fmt.Errorf("source-policy owner %s: process has not started", owner.name)
	}
	select {
	case <-owner.proc.Done():
		return fmt.Errorf("source-policy owner %s: process stopped", owner.name)
	default:
	}
	if err := owner.conn.Err(); err != nil {
		return fmt.Errorf("source-policy owner %s: %w", owner.name, err)
	}
	return nil
}
