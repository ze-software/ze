package fixture

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const supportDoctorOwnerCheckName = "ui/support-doctor-owner-check"

func init() {
	Register(supportDoctorOwnerCheckName, uiDriver(supportDoctorOwnerCheck))
}

func supportDoctorOwnerCheck(ctx context.Context) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(
		ctx,
		"ze",
		"support",
		"--module",
		"doctor",
		"--json",
		"--output",
		".",
		"missing-plugin.conf",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() != 0 {
			_, _ = os.Stderr.Write(stderr.Bytes())
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return productExitError{code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run ze support: %w", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		return fmt.Errorf(
			"support json decode failed: %w: %s",
			err,
			firstRunes(stdout.String(), 200),
		)
	}

	archivePath, ok := manifest["archive-path"].(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("support manifest missing archive-path: %v", manifest)
	}

	codes, err := supportDoctorCodes(archivePath)
	if err != nil {
		return err
	}

	for _, code := range codes {
		if code == "doctor-plugin-missing" {
			fmt.Println("OK support doctor owner check")
			return nil
		}
	}

	encodedCodes, marshalErr := json.Marshal(codes)
	if marshalErr != nil {
		return fmt.Errorf("support doctor.json missing owner-registered plugin check: codes=%v", codes)
	}
	return fmt.Errorf(
		"support doctor.json missing owner-registered plugin check: codes=%s",
		encodedCodes,
	)
}

type productExitError struct {
	code int
}

func (e productExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

// ExitCode allows the fixture runner to preserve the product's exit status.
func (e productExitError) ExitCode() int {
	return e.code
}

func supportDoctorCodes(archivePath string) ([]any, error) {
	archiveFile, err := os.Open(archivePath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, fmt.Errorf("open support archive: %w", err)
	}
	defer archiveFile.Close() //nolint:errcheck // fixture teardown

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return nil, fmt.Errorf("open support archive gzip stream: %w", err)
	}
	defer gzipReader.Close() //nolint:errcheck // fixture teardown

	tarReader := tar.NewReader(gzipReader)
	var doctorJSON []byte
	foundDoctor := false

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read support archive: %w", err)
		}
		if header.Name != "doctor.json" {
			continue
		}

		// Archive name lookup selects the last matching member. Keep replacing
		// the saved value so duplicate entries have the same behavior.
		foundDoctor = header.Typeflag == tar.TypeReg
		doctorJSON = nil
		if !foundDoctor {
			continue
		}

		doctorJSON, err = io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read doctor.json from support archive: %w", err)
		}
	}

	if !foundDoctor {
		return nil, errors.New("support archive missing doctor.json")
	}

	var doctor struct {
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal(doctorJSON, &doctor); err != nil {
		return nil, fmt.Errorf("decode support archive doctor.json: %w", err)
	}

	codes := make([]any, 0, len(doctor.Diagnostics))
	for _, diagnostic := range doctor.Diagnostics {
		codes = append(codes, diagnostic["code"])
	}
	return codes, nil
}

func firstRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
