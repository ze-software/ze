package hub

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
)

type recordingReconfigurable struct {
	addrs []string
	fail  error
	calls [][]string
}

func (r *recordingReconfigurable) Addresses() []string {
	return append([]string(nil), r.addrs...)
}

func (r *recordingReconfigurable) Reconfigure(_ context.Context, newAddrs []string) error {
	r.calls = append(r.calls, append([]string(nil), newAddrs...))
	if r.fail != nil {
		return r.fail
	}
	r.addrs = append([]string(nil), newAddrs...)
	return nil
}

func TestReloadListenersRollsBackAppliedServiceOnLaterFailure(t *testing.T) {
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	lg := &recordingReconfigurable{addrs: []string{"127.0.0.1:8443"}, fail: fmt.Errorf("lg refused")}
	migrator := NewListenerMigrator(nil)
	migrator.web = web
	migrator.lg = lg

	// VALIDATES: listener migration is all-or-revert inside a rejected reload.
	// PREVENTS: one service staying on the rejected address after a later service fails.
	err := migrator.ReloadListeners(context.Background(), listenerMigrationTree())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lg refused")

	assert.Equal(t, [][]string{{"127.0.0.1:3444"}, {"127.0.0.1:3443"}}, web.calls)
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
	assert.Equal(t, [][]string{{"127.0.0.1:8444"}}, lg.calls)
}

func listenerMigrationTree() *zeconfig.Tree {
	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	env.SetContainer("web", listenerServiceTree("3444"))
	env.SetContainer("looking-glass", listenerServiceTree("8444"))
	tree.SetContainer("environment", env)
	return tree
}

func listenerServiceTree(port string) *zeconfig.Tree {
	svc := zeconfig.NewTree()
	svc.Set("enabled", "true")
	srv := zeconfig.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	svc.AddListEntry("server", "main", srv)
	return svc
}
