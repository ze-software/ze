// Design: docs/guide/authentication.md -- bootstrap credential doctor through the shipped CLI.
// Related: storage_tree_fixture.go -- native command and store-root helpers.

package fixture

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const credentialDoctorInputMarker = "INVALID-PRIVATE-PAYLOAD"
const credentialDoctorConfig = "set system dns resolv-conf-path \"\"\n"

func init() {
	Register("storage/bootstrap-credential-doctor", bootstrapCredentialDoctorScenario)
}

// bootstrapCredentialDoctorScenario proves the registered check is exposed by
// the production CLI as stored input changes. It never calls the check itself.
func bootstrapCredentialDoctorScenario(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("storage/bootstrap-credential-doctor takes no arguments")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("bootstrap-functional-password"), bcrypt.MinCost)
	if err != nil {
		return err
	}
	store, err := createDaemonStorePopulated(os.Environ(), ".", func(store storage.Storage) error {
		for _, entry := range []struct {
			key   string
			value []byte
		}{
			{zefs.KeyLocalAdminUsername.Pattern, []byte(storageSeedUser)},
			{zefs.KeyLocalAdminPassword.Pattern, hash},
			{zefs.KeyLocalAdminAuthorizedKeys.Pattern, []byte(credentialDoctorInputMarker + "\n")},
		} {
			if err := store.WriteKey(entry.key, entry.value); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := store.Close(); err != nil {
		return err
	}
	if err := bootstrapCredentialDoctorJSON(ctx, true); err != nil {
		return err
	}
	if err := bootstrapCredentialFrameEquals(zefs.KeyLocalAdminAuthorizedKeys.Pattern, []byte(credentialDoctorInputMarker+"\n")); err != nil {
		return fmt.Errorf("doctor changed the refused credential: %w", err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		return err
	}
	if err := os.WriteFile("bootstrap-key.pub", ssh.MarshalAuthorizedKey(key), 0o600); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "write", zefs.KeyLocalAdminAuthorizedKeys.Pattern, "bootstrap-key.pub"); err != nil {
		return err
	}
	if err := bootstrapCredentialDoctorJSON(ctx, false); err != nil {
		return fmt.Errorf("valid key repair: %w", err)
	}
	if err := os.WriteFile("refused-key.pub", []byte(credentialDoctorInputMarker+"\n"), 0o600); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "write", zefs.KeyLocalAdminAuthorizedKeys.Pattern, "refused-key.pub"); err != nil {
		return err
	}
	if err := os.WriteFile("admin-disabled", []byte("true"), 0o600); err != nil {
		return err
	}
	if _, err := storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "write", zefs.KeyInstanceAdminDisabled.Pattern, "admin-disabled"); err != nil {
		return err
	}
	if err := bootstrapCredentialDoctorJSON(ctx, false); err != nil {
		return fmt.Errorf("explicit admin disable: %w", err)
	}
	if err := bootstrapCredentialFrameEquals(zefs.KeyLocalAdminAuthorizedKeys.Pattern, []byte(credentialDoctorInputMarker+"\n")); err != nil {
		return fmt.Errorf("disabled doctor probe changed the stored key: %w", err)
	}
	var status textbuf.Buffer
	return status.Str("OK: bootstrap credential doctor code, fix, privacy, repair and disable verified\n").StdOut()
}

// bootstrapCredentialDoctorJSON leaves unrelated host findings unconstrained.
// Doctor's normal status can be zero or one because those findings vary by
// host; process failures, malformed output and the exact owned row still fail.
func bootstrapCredentialDoctorJSON(ctx context.Context, want bool) error {
	// Standard input is explicitly local: askDaemonDoctor refuses to forward
	// "-", so this fixture never probes or answers from another running daemon.
	output, err := storageCommand(ctx, credentialDoctorConfig, "doctor", "--json", sourceConfig, "-")
	if err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			return fmt.Errorf("credential doctor could not run: %w", err)
		}
		if exited.ExitCode() != 1 {
			return fmt.Errorf("credential doctor exited unexpectedly: %w", err)
		}
	}
	if bytes.Contains(output, []byte(credentialDoctorInputMarker)) {
		return errors.New("credential doctor disclosed the refused key input")
	}
	var result diagnostic.DoctorResult
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(&result); err != nil {
		return fmt.Errorf("credential doctor returned invalid JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("credential doctor returned trailing output after its JSON result")
	}
	if result.SchemaVersion != diagnostic.SchemaVersion {
		return fmt.Errorf("credential doctor schema = %d, want %d", result.SchemaVersion, diagnostic.SchemaVersion)
	}
	found := 0
	for _, diag := range result.Diagnostics {
		if diag.Code != diagnostic.CodeDoctorLocalAdminCredentials {
			continue
		}
		found++
		if diag.Severity != diagnostic.SeverityWarning {
			return fmt.Errorf("credential doctor severity = %s, want warning", diag.Severity)
		}
		if !strings.HasPrefix(diag.Fix, "ze data --path ") {
			return fmt.Errorf("credential doctor has no explicit-path repair command: %q", diag.Fix)
		}
		if !strings.Contains(diag.Fix, " write ") {
			return fmt.Errorf("credential doctor does not name the credential restoration operation: %q", diag.Fix)
		}
		if diag.Actual == nil {
			return errors.New("credential doctor omitted the canonical reader's refusal")
		}
	}
	if want {
		if found != 1 {
			return fmt.Errorf("credential doctor emitted %d owned findings, want exactly one", found)
		}
		return nil
	}
	if found != 0 {
		return fmt.Errorf("repaired or explicitly disabled credentials retained %d owned findings", found)
	}
	return nil
}

func bootstrapCredentialFrameEquals(key string, expected []byte) error {
	store, err := openDaemonStoreReadOnly(os.Environ(), ".")
	if err != nil {
		return err
	}
	actual, readErr := store.ReadKey(key)
	if err := errors.Join(readErr, store.Close()); err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("credential frame %s changed during read-only doctor", key)
	}
	return nil
}
