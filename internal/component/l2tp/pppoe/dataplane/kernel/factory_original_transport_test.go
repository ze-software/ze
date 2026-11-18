// Design: docs/architecture/l2tp/bng-5-pppoe.md -- same original transport file
// Related: factory.go -- private setup port; transport.go -- original close

package kernel

import (
	"context"
	"errors"
	"os"
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestFactoryPassesExactOriginalTransport uses an owned pipe as a private
// custody witness and a trap numeric label. Public native constructors never
// receive the model's zero Created witnesses. The actual factory must forward
// and close the same original file, independently of its channel/unit lease.
func TestFactoryPassesExactOriginalTransport(t *testing.T) {
	original, peer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
	})
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := original.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	fixture := newFactoryFixture()
	fixture.ops.openTransport = func(context.Context, dataplane.Request) (*pppoxTransport, error) {
		return &pppoxTransport{file: original, fd: 991, closeFD: func(int) error { t.Error("numeric label authorized transport close"); return nil }}, nil
	}
	fixture.ops.setup = func(creator *pppkernel.UnitCreator, actual *os.File, name string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
		if actual != original {
			t.Fatalf("factory substituted original: %p", actual)
		}
		if creator != fixture.creator {
			t.Fatal("factory substituted creator")
		}
		if name != "" {
			t.Fatalf("factory substituted name: %q", name)
		}
		return fixture.channel, fixture.unit, nil
	}
	opened, err := openWithOps(t.Context(), factoryRequest(), fixture.ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := original.Stat(); err != nil {
		t.Fatalf("factory closed admitted transport: %v", err)
	}
	closed = true
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	if _, err := original.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original transport remains open: %v", err)
	}
}
