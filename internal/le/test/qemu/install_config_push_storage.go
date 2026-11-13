// Design: docs/architecture/testing/qemu-integration.md -- stopped-disk configuration publication.
// Related: install_storage.go -- the shared read-only partition extraction.
package testqemu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// configPushDisk MUST be called after the guest reboot has exited and joined.
// It reads only an owned partition copy; it never repairs the selected store.
func (installer *Installer) configPushDisk(ctx context.Context, work, disk, phase string) (content string, host ssh.PublicKey, resultErr error) {
	perm := filepath.Join(work, phase+"-perm.img")
	if _, _, err := installPerm(disk, perm); err != nil {
		return "", nil, err
	}
	extracted := filepath.Join(work, phase+"-store")
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return "", nil, err
	}
	var command textbuf.Buffer
	if err := installer.storageDebugFS(ctx, perm, command.Str("rdump /ze ").Str(strconv.Quote(extracted)).String(), false); err != nil {
		return "", nil, err
	}
	root := filepath.Join(extracted, "ze")
	store, err := storage.OpenReadOnly(root, root)
	if err != nil {
		return "", nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	// ReadActiveConfig permits a legacy mirror only without a pointer. This
	// proof requires the selected version, and therefore checks the pointer first.
	if _, err := store.ReadKey(zefs.KeyConfigActive.Key("ze.conf")); err != nil {
		return "", nil, errors.New("stopped disk has no readable active configuration pointer")
	}
	selected, err := storage.ReadActiveConfig(store, "ze.conf")
	if err != nil {
		return "", nil, err
	}
	keyPath := filepath.Join(root, storage.TreeName, filepath.FromSlash(zefs.KeyFileActive.Key(configPushHostKey)))
	frame, err := os.ReadFile(keyPath) //nolint:gosec // Host-key frame on the stopped owned disk.
	if err != nil {
		return "", nil, err
	}
	defer clear(frame)
	private, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
	if err != nil {
		return "", nil, errors.New("stopped disk SSH host-key frame is invalid")
	}
	if next != len(frame) {
		return "", nil, errors.New("stopped disk SSH host-key frame has trailing data")
	}
	signer, err := ssh.ParsePrivateKey(private)
	if err != nil {
		return "", nil, errors.New("stopped disk SSH host key is invalid")
	}
	return string(selected), signer.PublicKey(), nil
}
