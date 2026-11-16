// Design: docs/architecture/l2tp/bng-5-pppoe.md -- synthetic admission ownership

package pppoe

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// This owner is used only where the production driver is unstarted or the
// table is populated directly. Unexpected data-plane operations deliberately
// reach the nil embedded contract instead of succeeding as fake kernel work.
type fixtureSessionHandle struct {
	ppp.SessionHandle
	identity ppp.SessionIdentity
}

func (h *fixtureSessionHandle) Identity() ppp.SessionIdentity { return h.identity }
func (*fixtureSessionHandle) Close() error                    { return nil }

type fixtureFrames struct{ bytes.Buffer }

func (f *fixtureFrames) Control() io.ReadWriter { return &f.Buffer }
func (*fixtureFrames) Opened() io.Reader        { return nil }
func (*fixtureFrames) InterruptRead() error {
	return errors.New("synthetic admission has no running reader")
}

type fixtureEmptyTransport struct{}

func (fixtureEmptyTransport) Close() error { return nil }

func fixtureOpened(t *testing.T, identity string) dataplane.OpenedSession {
	t.Helper()
	handle := &fixtureSessionHandle{identity: ppp.SessionIdentity{InterfaceName: identity}}
	opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, handle, fixtureEmptyTransport{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opened.Dataplane().Close(); _ = opened.CloseTransport() })
	return opened
}
