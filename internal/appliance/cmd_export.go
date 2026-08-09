// Design: docs/architecture/appliance/disaster-recovery.md -- bastion disaster recovery export

package appliance

import (
	"archive/tar"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

var errExportRequiresEncryptionPassphraseArchivesAlways = errors.New("export requires encryption passphrase (archives always encrypted)")

func init() {
	cmdExport = runExport
}

func runExport(args []string) int {
	fs := flag.NewFlagSet("appliance export", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Export all appliances into a single archive")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance export [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	dir := getBaseDir()

	passphrase, err := resolveExportPassphrase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	defer ZeroBytes(passphrase)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *allFlag {
		archivePath, exportErr := exportAll(dir, passphrase, cwd)
		if exportErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", exportErr)
			return exitError
		}
		fmt.Printf("exported to %s (encrypted)\n", archivePath)
		return exitOK
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	name := fs.Arg(0)
	archivePath, exportErr := exportAppliance(dir, name, passphrase, cwd)
	if exportErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", exportErr)
		return exitError
	}
	fmt.Printf("exported to %s (encrypted)\n", archivePath)
	return exitOK
}

func resolveExportPassphrase() ([]byte, error) {
	passphrase, _, err := ResolvePassphrase(func() ([]byte, error) {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, errExportRequiresEncryptionPassphraseArchivesAlways
		}
		fmt.Fprint(os.Stderr, "Archive encryption passphrase: ")
		pass, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if readErr != nil {
			return nil, readErr
		}
		if len(pass) == 0 {
			return nil, errExportRequiresEncryptionPassphraseArchivesAlways
		}
		return pass, nil
	})
	if err != nil {
		return nil, errExportRequiresEncryptionPassphraseArchivesAlways
	}
	if len(passphrase) == 0 {
		return nil, errExportRequiresEncryptionPassphraseArchivesAlways
	}
	return passphrase, nil
}

func exportAppliance(baseDir, name string, passphrase []byte, outDir string) (string, error) {
	if len(passphrase) == 0 {
		return "", errExportRequiresEncryptionPassphraseArchivesAlways
	}

	appDir := AppliancePath(baseDir, name)
	if _, err := LoadConfig(ConfigPath(baseDir, name)); err != nil {
		return "", fmt.Errorf("appliance %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tarAppliance(&buf, appDir, name); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}

	encrypted, err := Encrypt(buf.Bytes(), passphrase)
	if err != nil {
		return "", fmt.Errorf("encrypt archive: %w", err)
	}

	archivePath := filepath.Join(outDir, name+".ze.enc")
	if err := os.WriteFile(archivePath, encrypted, 0o600); err != nil {
		return "", fmt.Errorf("write archive: %w", err)
	}

	return archivePath, nil
}

func exportAll(baseDir string, passphrase []byte, outDir string) (string, error) {
	if len(passphrase) == 0 {
		return "", errExportRequiresEncryptionPassphraseArchivesAlways
	}

	names, err := listAppliances(baseDir)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no appliances found in %s", baseDir)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		appDir := AppliancePath(baseDir, name)
		if err := tarApplianceInto(tw, appDir, name); err != nil {
			tw.Close() //nolint:errcheck // cleanup on error
			return "", fmt.Errorf("archive %s: %w", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("close archive: %w", err)
	}

	encrypted, err := Encrypt(buf.Bytes(), passphrase)
	if err != nil {
		return "", fmt.Errorf("encrypt archive: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	archivePath := filepath.Join(outDir, fmt.Sprintf("appliances-%s.ze.enc", ts))
	if err := os.WriteFile(archivePath, encrypted, 0o600); err != nil {
		return "", fmt.Errorf("write archive: %w", err)
	}

	return archivePath, nil
}

func listAppliances(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", baseDir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == sharedDirName || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, loadErr := LoadConfig(ConfigPath(baseDir, e.Name())); loadErr == nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func tarAppliance(w io.Writer, appDir, name string) error {
	tw := tar.NewWriter(w)
	if err := tarApplianceInto(tw, appDir, name); err != nil {
		tw.Close() //nolint:errcheck // cleanup on error
		return err
	}
	return tw.Close()
}

func tarApplianceInto(tw *tar.Writer, appDir, name string) error {
	return filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(appDir, path)
		if relErr != nil {
			return relErr
		}

		if shouldExcludeFromExport(rel, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		tarPath := filepath.Join(name, rel)

		header, headerErr := tar.FileInfoHeader(info, "")
		if headerErr != nil {
			return headerErr
		}
		header.Name = tarPath

		if writeErr := tw.WriteHeader(header); writeErr != nil {
			return writeErr
		}

		if info.IsDir() {
			return nil
		}

		f, openErr := os.Open(path) //nolint:gosec // walking appliance dir
		if openErr != nil {
			return openErr
		}
		defer f.Close() //nolint:errcheck // read-only

		_, copyErr := io.Copy(tw, f)
		return copyErr
	})
}

func shouldExcludeFromExport(rel string, info os.FileInfo) bool {
	base := filepath.Base(rel)

	if !info.IsDir() {
		if strings.HasSuffix(base, ".img") || strings.HasSuffix(base, ".img.sha256") {
			return true
		}
		if base == databaseFileName {
			return true
		}
	}

	return false
}
