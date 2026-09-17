// Design: docs/architecture/testing/qemu-integration.md -- appliance seed import proof
package qemu

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

const installImportMark = "gokrazy: imported seed into live store"

func installSeedCertificate(seed string) (certificate []byte, resultErr error) {
	store, err := storage.OpenBlob(seed, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	data, err := store.ReadKey(zefs.KeyWebCert.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read appliance seed certificate: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("appliance seed has no PEM certificate")
	}
	if block.Type != "CERTIFICATE" {
		return nil, errors.New("appliance seed web material is not a certificate")
	}
	return block.Bytes, nil
}

func installSeedTLS(ctx context.Context, port int, certificate []byte) (resultErr error) {
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dialer := tls.Dialer{Config: &tls.Config{
		// #nosec G402 -- the exact expected certificate is checked below.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}}
	connection, err := dialer.DialContext(deadline, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("seeded web listener: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()
	tlsConnection, ok := connection.(*tls.Conn)
	if !ok {
		return fmt.Errorf("seeded web listener: dialer returned %T, want *tls.Conn", connection)
	}
	peer := tlsConnection.ConnectionState().PeerCertificates
	if len(peer) == 0 {
		return errors.New("seeded web listener sent no certificate")
	}
	if !bytes.Equal(peer[0].Raw, certificate) {
		return errors.New("web listener certificate differs from the seed")
	}
	return nil
}

// installPerm copies the fourth GPT partition while the target VM is stopped.
// The caller MUST finish all edits before starting QEMU on the disk again.
func installPerm(disk, target string) (offset, size int64, resultErr error) {
	entries, err := installGPTEntries(disk)
	if err != nil {
		return 0, 0, err
	}
	if len(entries) < 4 {
		return 0, 0, errors.New("installed disk has no /perm partition")
	}
	entry := entries[3]
	// #nosec G304 -- disk is the installer-owned target image.
	input, err := os.Open(disk)
	if err != nil {
		return 0, 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	info, err := input.Stat()
	if err != nil {
		return 0, 0, err
	}
	sectors := uint64(info.Size() / 512)
	if entry.Last < entry.First {
		return 0, 0, errors.New("invalid /perm partition bounds")
	}
	if entry.Last >= sectors {
		return 0, 0, errors.New("/perm partition exceeds disk image")
	}
	offset, size = int64(entry.First)*512, int64(entry.Last-entry.First+1)*512
	// #nosec G304 -- target is constructed beneath the installer-owned work directory.
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	_, err = io.CopyN(output, io.NewSectionReader(input, offset, size), size)
	return offset, size, err
}

func (installer *Installer) storageDebugFS(ctx context.Context, perm, command string, writable bool) error {
	tool, err := installer.ops.Look("debugfs")
	if err != nil {
		tool = installer.brewDebugfs()
		if tool == "" {
			return errors.New("appliance storage proof requires debugfs (install e2fsprogs)")
		}
	}
	args := []string{"-R", command, perm}
	if writable {
		args = append([]string{"-w"}, args...)
	}
	result, err := installer.run(ctx, commandSpec{Name: tool, Args: args, Env: installer.ops.Environ()})
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("debugfs %s: %s%s", command, result.Stdout, result.Stderr)
	}
	return nil
}

// seedInterruptedImport places an unfinished staging tree beside the real seed.
// It exercises boot recovery through gokrazyAutoInit without a test-only daemon
// switch. Core storage tests cover interruption after live-tree publication.
func (installer *Installer) seedInterruptedImport(ctx context.Context, work, disk string) (resultErr error) {
	perm := filepath.Join(work, "import-perm.img")
	offset, size, err := installPerm(disk, perm)
	if err != nil {
		return err
	}
	partial := filepath.Join(work, "partial-frame")
	if err := os.WriteFile(partial, []byte("unfinished frame"), 0o600); err != nil {
		return err
	}
	for _, command := range []string{
		"mkdir /ze/database.import-tmp-interrupted",
		fmt.Sprintf("write %q /ze/database.import-tmp-interrupted/partial", partial),
	} {
		if err := installer.storageDebugFS(ctx, perm, command, true); err != nil {
			return err
		}
	}
	// debugfs can exit successfully after a failed command. Read the fixture
	// back before using it as evidence of an interrupted import.
	probe := filepath.Join(work, "partial-readback")
	if err := installer.storageDebugFS(ctx, perm, fmt.Sprintf("dump /ze/database.import-tmp-interrupted/partial %q", probe), false); err != nil {
		return err
	}
	// #nosec G304 -- probe is a path this run made under its work directory.
	data, err := os.ReadFile(probe)
	if err != nil {
		return err
	}
	if string(data) != "unfinished frame" {
		return errors.New("debugfs did not write the interrupted import fixture")
	}
	// #nosec G304 -- perm is the /perm extraction this run made under its work directory.
	input, err := os.Open(perm)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	// #nosec G304 -- disk is the installer-owned target image.
	output, err := os.OpenFile(disk, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	if _, err := output.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(output, input, size); err != nil {
		return err
	}
	return output.Sync()
}

// assertImportedSeed runs only after bootTargetSSH has stopped the VM. It checks
// on-disk results independently of the success log and SSH authentication.
func (installer *Installer) assertImportedSeed(ctx context.Context, work, disk, seed, serialPath string) (resultErr error) {
	// #nosec G304 -- serialPath is the serial log this run wrote under its work directory.
	serial, err := os.ReadFile(serialPath)
	if err != nil {
		return err
	}
	if !bytes.Contains(serial, []byte(installImportMark)) {
		return errors.New("first boot did not log the explicit seed import")
	}
	perm := filepath.Join(work, "import-perm.img")
	if _, _, err := installPerm(disk, perm); err != nil {
		return err
	}
	extracted := filepath.Join(work, "import-result")
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return err
	}
	if err := installer.storageDebugFS(ctx, perm, fmt.Sprintf("rdump /ze %q", extracted), false); err != nil {
		return err
	}
	root := filepath.Join(extracted, "ze")
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	retired := false
	for _, entry := range entries {
		if entry.Name() == "database.zefs" {
			return errors.New("first boot left the live seed beside the tree")
		}
		if strings.HasPrefix(entry.Name(), "database.zefs.replaced-") {
			retired = entry.Type().IsRegular()
		}
	}
	if !retired {
		return errors.New("first boot did not retain a retired seed")
	}
	source, err := storage.OpenBlob(seed, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, source.Close()) }()
	keys, err := source.ListKeys("")
	if err != nil {
		return err
	}
	for _, key := range keys {
		want, err := source.ReadKey(key)
		if err != nil {
			return err
		}
		// #nosec G304 -- root is the /perm extraction this run made under its work directory; key comes from the seed.
		frame, err := os.ReadFile(filepath.Join(root, "database", filepath.FromSlash(key)))
		if err != nil {
			return fmt.Errorf("imported key %s: %w", key, err)
		}
		got, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
		if err != nil {
			return fmt.Errorf("imported key %s: %w", key, err)
		}
		if next != len(frame) {
			return fmt.Errorf("imported key %s has trailing frame data", key)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("imported key %s differs from seed", key)
		}
	}
	return nil
}
