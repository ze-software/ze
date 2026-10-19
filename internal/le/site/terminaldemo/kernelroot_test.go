// Related: render.go -- Engine.checkKernel and Engine.demoGrants, the docker
// queries these tests root.

package siteterminaldemo

import (
	"context"
	"errors"
	"testing"
)

// VALIDATES: the kernel check and the net-sysctl grant query run under the
// engine's root context, so a Ctrl-C reaching `./le site terminal-demo`'s
// signal root ends them.
// METHOD: an engine whose root is already canceled, with checks that answer
// the context they were handed. Rooted on context.Background, they answer nil.
// PREVENTS: a Ctrl-C the docker client never sees, because it heads a process
// group of its own, leaving the probe to run to its bound.
func TestKernelQueriesRunUnderTheEngineRoot(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	engine := New(Options{
		Root:        t.TempDir(),
		Context:     canceled,
		KernelCheck: func(ctx context.Context, _ string) error { return ctx.Err() },
		NetSysctl:   func(ctx context.Context) ([]string, error) { return nil, ctx.Err() },
	})

	if err := engine.checkKernel(); !errors.Is(err, context.Canceled) {
		t.Errorf("kernel check under a canceled root answered %v, want context.Canceled", err)
	}
	if _, err := engine.demoGrants(&Demo{ID: "sysctl", NetSysctl: new(true)}); !errors.Is(err, context.Canceled) {
		t.Errorf("net-sysctl query under a canceled root answered %v, want context.Canceled", err)
	}
}
