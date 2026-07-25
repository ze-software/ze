// Design: plan/learned/676-appliance-3-recovery.md — bastion disaster recovery import

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

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/cliio"
)

var (
	errImportRequiresDecryptionPassphrase    = errors.New("import requires decryption passphrase")
	errArchiveContainsNoApplianceDirectories = errors.New("archive contains no appliance directories")
)

func init() {
	cmdImport = runImport
}

func runImport(args []string) int {
	fs := flag.NewFlagSet("appliance import", flag.ContinueOnError)
	dirFlag := fs.String("dir", "", "Target directory for import (default: appliance base dir)")
	forceFlag := fs.Bool("force", false, "Overwrite existing appliance directories without prompting")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance import [options] <archive>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <archive>\n")
		fs.Usage()
		return exitError
	}

	archivePath := fs.Arg(0)

	passphrase, err := resolveImportPassphrase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	defer ZeroBytes(passphrase)

	targetDir := getBaseDir()
	if *dirFlag != "" {
		targetDir = *dirFlag
	}

	imported, importErr := importArchive(archivePath, passphrase, targetDir, *forceFlag)
	if importErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", importErr)
		return exitError
	}

	fmt.Printf("imported %d appliance(s) to %s\n", len(imported), targetDir)
	return exitOK
}

func resolveImportPassphrase() ([]byte, error) {
	passphrase, _, err := ResolvePassphrase(func() ([]byte, error) {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, errImportRequiresDecryptionPassphrase
		}
		fmt.Fprint(os.Stderr, "Archive decryption passphrase: ")
		pass, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if readErr != nil {
			return nil, readErr
		}
		return pass, nil
	})
	return passphrase, err
}

func importArchive(archivePath string, passphrase []byte, targetDir string, force bool) ([]string, error) {
	encrypted, err := cliio.ReadFile(archivePath) // "-" reads stdin
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}

	tarBytes, err := Decrypt(encrypted, passphrase)
	if err != nil {
		return nil, err
	}

	names, err := validateArchiveStructure(tarBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	if !force {
		for _, name := range names {
			dst := filepath.Join(targetDir, name)
			if _, statErr := os.Stat(dst); statErr == nil {
				return nil, fmt.Errorf("appliance %q already exists at %s (use --force to overwrite)", name, dst)
			}
		}
	}

	if err := extractTar(tarBytes, targetDir); err != nil {
		return nil, fmt.Errorf("extract archive: %w", err)
	}

	for _, name := range names {
		cfgPath := filepath.Join(targetDir, name, configFileName)
		if _, loadErr := LoadConfig(cfgPath); loadErr != nil {
			fmt.Fprintf(os.Stderr, "warning: restored %s has invalid config: %v\n", name, loadErr)
		}
	}

	return names, nil
}

func validateArchiveStructure(tarBytes []byte) ([]string, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	seen := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if strings.Contains(clean, "..") {
			return nil, fmt.Errorf("path traversal in archive: %s", hdr.Name)
		}

		parts := strings.SplitN(clean, string(filepath.Separator), 2)
		if len(parts) > 0 {
			topDir := parts[0]
			if len(parts) == 2 && parts[1] == configFileName {
				seen[topDir] = true
			}
			if !seen[topDir] {
				seen[topDir] = false
			}
		}
	}

	var names []string
	for name, hasConfig := range seen {
		if !hasConfig {
			return nil, fmt.Errorf("directory %q missing %s", name, configFileName)
		}
		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, errArchiveContainsNoApplianceDirectories
	}

	return names, nil
}

func extractTar(tarBytes []byte, targetDir string) error {
	tr := tar.NewReader(bytes.NewReader(tarBytes))

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if strings.Contains(clean, "..") {
			return fmt.Errorf("path traversal in archive: %s", hdr.Name)
		}

		target := filepath.Join(targetDir, clean) //nolint:gosec // clean verified above

		switch hdr.Typeflag {
		case tar.TypeDir:
			if mkErr := os.MkdirAll(target, os.FileMode(hdr.Mode)); mkErr != nil {
				return mkErr
			}
		case tar.TypeReg:
			if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
				return mkErr
			}
			f, createErr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)) //nolint:gosec // path traversal checked above
			if createErr != nil {
				return createErr
			}
			if _, copyErr := io.Copy(f, io.LimitReader(tr, 100*1024*1024)); copyErr != nil { //nolint:mnd // 100 MiB per-file limit
				f.Close() //nolint:errcheck // cleanup on error
				return copyErr
			}
			if closeErr := f.Close(); closeErr != nil {
				return closeErr
			}
		default:
			// Reject symlinks, hardlinks, devices, FIFOs explicitly. A ze
			// export archive contains only directories and regular files, so a
			// crafted archive with a symlink (a directory-escape vector) is
			// refused rather than silently skipped.
			return fmt.Errorf("unsupported tar entry type %q in archive: %s", hdr.Typeflag, hdr.Name)
		}
	}

	return nil
}
