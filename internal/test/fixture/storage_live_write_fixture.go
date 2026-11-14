// Design: docs/architecture/zefs-format.md -- the ze data CLI and its daemon forms
// Related: storage_live_fixture.go -- storageLiveClient, storageLiveRequest, storageLiveRefused
// Related: register_storage_live.go -- registers storage/data-write-live

package fixture

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

// storageLiveWriteKey is the runtime-state key data-write-live.ci writes
// through the running daemon. storageLiveWriteSecret is the first value it
// writes, standing for a credential: its base64 (c2VjcmV0LWNyZWRlbnRpYWw=) must
// reach no record, and the .ci rejects it on the whole run's stderr.
const (
	storageLiveWriteKey    = "meta/test/data-write"
	storageLiveWriteSecret = "secret-credential"
)

// The content bound of update data key, as ze-data-cmd.yang declares it: one
// SSH packet with the command around it.
const (
	storageLiveContentEncodedMax = 261120
	storageLiveContentBytesMax   = 195840
	storageLiveBoundText         = "195,840 bytes"
)

// storageLiveWriteLocally proves AC-7, AC-8 and AC-12 on a running daemon,
// over the real SSH route. The local `ze data write` beside the daemon creates
// the key and then replaces it, each time with exactly the local route's line
// and exit 0, and a fresh reader handle reads exactly the file's bytes; a file
// at the bound travels and is read back, one byte more is refused naming the
// bound and leaves the key as it was. `update data key` sent over SSH exec with
// a content at the bound is written; one character more is refused naming the
// bound; a configuration key is refused. `show audit` then holds the write
// with the placeholder and never the secret's base64.
func storageLiveWriteLocally(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	// The daemon's own store, in the runner's config folder, with the login
	// the client store holds: the environment an operator's shell on the
	// daemon's host has.
	dataEnv := overrideEnv04(os.Environ(),
		"ZE_SSH_PASSWORD=testpass",
		"ZE_SSH_HOST="+addrLoopback,
		"ZE_SSH_PORT="+args[0],
		"ZE_SSH_USERNAME="+storageSeedUser,
		"NO_COLOR=1",
	)
	for _, value := range [][]byte{[]byte(storageLiveWriteSecret), {0x00, 0xff, 0x10, '\n', 0x80}} {
		if err := storageLiveWriteThrough(ctx, dataEnv, value); err != nil {
			return err
		}
	}
	if err := storageLiveWriteBound(ctx, dataEnv); err != nil {
		return err
	}
	if err := storageLiveUpdateOverSSH(args[0]); err != nil {
		return err
	}
	if err := storageLiveRefused(ctx, env, "update data key file/active/data-write-live.conf content c3RhdGU=",
		"is configuration, not runtime state"); err != nil {
		return err
	}
	if err := storageLiveWriteAudited(ctx, env); err != nil {
		return err
	}
	return storageLiveOK("data-write-live")
}

// storageLiveWriteThrough runs the local `ze data write` of value beside the
// daemon and requires the local route's one line, exit 0, and the key holding
// exactly value when a fresh reader handle reads it.
func storageLiveWriteThrough(ctx context.Context, env []string, value []byte) error {
	output, code, err := storageLiveDataWrite(ctx, env, value)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ze data write of %d bytes exited %d, want 0:\n%s", len(value), code, output)
	}
	want := "wrote " + storageLiveWriteKey + " (" + strconv.Itoa(len(value)) + " bytes) into "
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 1 {
		return fmt.Errorf("ze data write printed %d lines, want the local route's one:\n%s", len(lines), output)
	}
	if !strings.HasPrefix(lines[0], want) {
		return fmt.Errorf("ze data write printed %q, want a line starting %q", lines[0], want)
	}
	return storageLiveHoldsValue(value)
}

// storageLiveWriteBound requires a file of 195,840 bytes to travel through the
// local route and be read back, and one byte more to be refused naming the
// bound, exit 2, with the key still holding the file at the bound.
func storageLiveWriteBound(ctx context.Context, env []string) error {
	atBound := bytes.Repeat([]byte{'z'}, storageLiveContentBytesMax)
	if err := storageLiveWriteThrough(ctx, env, atBound); err != nil {
		return err
	}
	output, code, err := storageLiveDataWrite(ctx, env, bytes.Repeat([]byte{'y'}, storageLiveContentBytesMax+1))
	if err != nil {
		return err
	}
	if code != 2 {
		return fmt.Errorf("ze data write over the bound exited %d, want 2:\n%s", code, output)
	}
	for _, want := range []string{storageLiveBoundText, "ze data write <key> <file> while ze is stopped"} {
		if !strings.Contains(output, want) {
			return fmt.Errorf("ze data write over the bound lacks %q:\n%s", want, output)
		}
	}
	return storageLiveHoldsValue(atBound)
}

// storageLiveUpdateOverSSH sends `update data key` over SSH exec, the route a
// remote operator takes, with a content of 261,120 characters, and requires
// the key to hold its 195,840 decoded bytes; a content one character longer
// is refused naming the bound.
func storageLiveUpdateOverSSH(port string) error {
	creds := sshclient.Credentials{Host: addrLoopback, Port: port, Username: storageSeedUser, Auth: valueTestPassword}
	atBound := strings.Repeat("QUJD", storageLiveContentEncodedMax/4)
	output, err := sshclient.ExecCommand(creds, "update data key "+storageLiveWriteKey+" content "+atBound)
	if err != nil {
		return fmt.Errorf("update data key with %d characters over SSH exec: %w", storageLiveContentEncodedMax, err)
	}
	if !strings.Contains(output, strconv.Itoa(storageLiveContentBytesMax)) {
		return fmt.Errorf("update data key answer lacks the byte count %d:\n%s", storageLiveContentBytesMax, output)
	}
	if err := storageLiveHoldsValue(bytes.Repeat([]byte("ABC"), storageLiveContentBytesMax/3)); err != nil {
		return err
	}
	_, err = sshclient.ExecCommand(creds, "update data key "+storageLiveWriteKey+" content "+atBound+"QUJD")
	if err == nil {
		return errors.New("update data key one block over the bound was not refused")
	}
	if !strings.Contains(err.Error(), storageLiveBoundText) {
		return fmt.Errorf("update data key over the bound refused without %q: %w", storageLiveBoundText, err)
	}
	return nil
}

// storageLiveWriteAudited requires `show audit` to hold the daemon's writes
// under data-key-change with the placeholder in place of the content, and
// never the base64 of the secret written first.
func storageLiveWriteAudited(ctx context.Context, env []string) error {
	output, err := storageLiveRequest(ctx, env, "show audit action data-key-change")
	if err != nil {
		return fmt.Errorf("show audit: %w:\n%s", err, output)
	}
	want := "update data key " + storageLiveWriteKey + " content <redacted>"
	if !strings.Contains(output, want) {
		return fmt.Errorf("show audit lacks %q:\n%s", want, output)
	}
	if strings.Contains(output, base64.StdEncoding.EncodeToString([]byte(storageLiveWriteSecret))) {
		return errors.New("show audit holds the base64 of the secret")
	}
	return nil
}

// storageLiveHoldsValue requires a fresh reader handle on the daemon's store
// to read exactly value under storageLiveWriteKey.
func storageLiveHoldsValue(value []byte) error {
	tree, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return err
	}
	stored, err := tree.ReadKey(storageLiveWriteKey)
	if closeErr := tree.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return fmt.Errorf("read %s back: %w", storageLiveWriteKey, err)
	}
	if !bytes.Equal(stored, value) {
		return fmt.Errorf("%s holds %d bytes, not the %d bytes written", storageLiveWriteKey, len(stored), len(value))
	}
	return nil
}

// storageLiveDataWrite writes value to a file and runs the local
// `ze data write <key> <file>`, answering its output (stdout and stderr
// together) and its exit code. An error is a run that did not reach an exit
// code, never a refusal.
func storageLiveDataWrite(ctx context.Context, env []string, value []byte) (string, int, error) {
	path, err := filepath.Abs("data-write-value.bin")
	if err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(path, value, 0o600); err != nil {
		return "", 0, err
	}
	output, err := runCommandProcess04(ctx, env, nil, "data", "write", storageLiveWriteKey, path)
	if err == nil {
		return output, 0, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output, exitErr.ExitCode(), nil
	}
	return output, 0, err
}
