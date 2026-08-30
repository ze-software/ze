package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func init() {
	Register("ui/doctor-web-tls-pair", uiDriver(runUIDoctorWebTLSPair))
}

func runUIDoctorWebTLSPair(ctx context.Context) error {
	if err := os.MkdirAll("meta/web", 0o750); err != nil {
		return fmt.Errorf("mkdir meta/web: %w", err)
	}
	if err := copyUIDoctorWebTLSPairFile("cert.pem", "meta/web/cert"); err != nil {
		return err
	}
	if err := copyUIDoctorWebTLSPairFile("foreign.key", "meta/web/key"); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "ze", "doctor", "--json", "web.conf")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
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

func copyUIDoctorWebTLSPairFile(source, destination string) error {
	input, err := os.Open(source) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer input.Close() //nolint:errcheck // fixture teardown

	info, err := input.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", source, err)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("open %s: %w", destination, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy %s to %s: %w", source, destination, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destination, err)
	}
	return nil
}
