package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func init() {
	Register("ui/doctor-web-tls-pair", uiDriver(runUIDoctorWebTLSPair))
}

// runUIDoctorWebTLSPair seeds the mismatched pair into the tree store the
// doctor opens under the working directory, then runs the doctor on it.
func runUIDoctorWebTLSPair(ctx context.Context) error {
	cert, err := os.ReadFile("cert.pem")
	if err != nil {
		return fmt.Errorf("read cert.pem: %w", err)
	}
	key, err := os.ReadFile("foreign.key")
	if err != nil {
		return fmt.Errorf("read foreign.key: %w", err)
	}
	store, err := storage.CreatePopulated(".", func(seed storage.Storage) error {
		if err := seed.WriteFile(zefs.KeyWebCert.Pattern, cert, 0); err != nil {
			return err
		}
		return seed.WriteFile(zefs.KeyWebKey.Pattern, key, 0)
	})
	if err != nil {
		return fmt.Errorf("seed the web TLS pair: %w", err)
	}
	if err := store.Close(); err != nil {
		return fmt.Errorf("close the seeded store: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ze", "doctor", "--json", "web.conf")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	fmt.Fprint(os.Stdout, stdout.String()) //nolint:errcheck // progress output
	fmt.Fprint(os.Stderr, stderr.String())
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return fmt.Errorf("ze doctor exit=%w, want 1: %s%s", err, stdout.String(), stderr.String())
	}
	for _, expected := range []string{"doctor-tls-invalid", "certificate and key in storage are not a usable pair"} {
		if !strings.Contains(stdout.String(), expected) {
			return fmt.Errorf("ze doctor output does not contain %q: %s", expected, stdout.String())
		}
	}
	fmt.Println("OK: mismatched TLS pair diagnosed")
	return nil
}
