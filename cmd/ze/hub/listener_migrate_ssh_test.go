// Design: docs/guide/configuration.md -- an SSH listener change applies on reload, all or none
// Related: listener_migrate.go -- buildChanges carries the SSH change, migrateListeners rolls back

package hub

import (
	"context"
	"errors"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sshReloadTree is a reloaded config that moves web to port 3444 and gives SSH
// the server entries named, each as "ip port".
func sshReloadTree(entries map[string][2]string) *zeconfig.Tree {
	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	env.SetContainer("web", listenerServiceTree("3444"))
	sshBlock := zeconfig.NewTree()
	for key, ipPort := range entries {
		srv := zeconfig.NewTree()
		srv.Set("ip", ipPort[0])
		srv.Set("port", ipPort[1])
		sshBlock.AddListEntry("server", key, srv)
	}
	env.SetContainer("ssh", sshBlock)
	tree.SetContainer("environment", env)
	return tree
}

// TestReloadSSHListenerBindFailureKeepsPrevious drives a reload whose SSH list
// cannot bind through the real migrator. Method: web is applied first and must
// be rolled back; the SSH fake refuses its new set, as Server.Reconfigure does
// when an address is not on the host, and keeps its old one.
// VALIDATES: AC-11 -- the reload is refused naming the address, SSH keeps its
// previous listeners, and the service migrated earlier is back on its addresses.
// PREVENTS: an SSH listener change accepted at commit and applied to nothing, or
// a failed SSH move leaving another service on the rejected config.
func TestReloadSSHListenerBindFailureKeepsPrevious(t *testing.T) {
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	sshSrv := &recordingReconfigurable{
		addrs: []string{"127.0.0.1:2222"},
		fail:  errors.New("bind SSH server on 192.0.2.1:2222: cannot assign requested address"),
	}
	migrator := newListenerMigrator()
	migrator.web = web
	migrator.setSSH(sshSrv)

	_, err := migrator.reloadListeners(context.Background(), sshReloadTree(map[string][2]string{
		"a": {"127.0.0.1", "2222"},
		"b": {"192.0.2.1", "2222"},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "192.0.2.1:2222")
	assert.Contains(t, err.Error(), "reconfigure ssh")

	require.Len(t, sshSrv.calls, 1, "the SSH change must reach the server")
	assert.ElementsMatch(t, []string{"127.0.0.1:2222", "192.0.2.1:2222"}, sshSrv.calls[0])
	assert.Equal(t, []string{"127.0.0.1:2222"}, sshSrv.addrs, "SSH keeps its previous listeners")
	assert.Equal(t, [][]string{{"127.0.0.1:3444"}, {"127.0.0.1:3443"}}, web.calls,
		"web is migrated, then rolled back when SSH fails")
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
}

// TestReloadSSHListenerMoves drives a reload that moves SSH from 127.0.0.1 to
// ::1 through the real migrator and the real SSH config extraction.
// VALIDATES: AC-12 -- SSH serves the new list after the reload, with no restart;
// the IPv6 address reaches the server bracketed, the form net.Listen takes.
// PREVENTS: the SSH listener list being read only at start.
func TestReloadSSHListenerMoves(t *testing.T) {
	sshSrv := &recordingReconfigurable{addrs: []string{"127.0.0.1:2222"}}
	migrator := newListenerMigrator()
	migrator.setSSH(sshSrv)

	tree := sshReloadTree(map[string][2]string{"a": {"::1", "2222"}})
	_, err := migrator.reloadListeners(context.Background(), tree)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"[::1]:2222"}}, sshSrv.calls)
	assert.Equal(t, []string{"[::1]:2222"}, sshSrv.addrs)

	// The same list again is no change, so the server is not called.
	_, err = migrator.reloadListeners(context.Background(), tree)
	require.NoError(t, err)
	assert.Len(t, sshSrv.calls, 1)
}
