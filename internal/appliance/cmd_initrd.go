// Design: docs/architecture/appliance/build-artifacts.md -- installer initrd download/build

package appliance

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultInitrdVersion = "v2"
	initrdURLKey         = "ze.appliance.initrd.url"
	initrdToolsDir       = "build/initrd"
	// initrdCommand is how an operator spells this command on the command line.
	initrdCommand = "ze appliance initrd"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         initrdURLKey,
	Type:        envTypeString,
	Description: "Base URL for pre-built installer initrd downloads",
})

var (
	initrdMakeBuildFn = defaultInitrdMakeBuild
	initrdLookPathFn  = exec.LookPath
)

func init() {
	cmdInitrd = runInitrd
}

func runInitrd(args []string) int {
	fs := flag.NewFlagSet("appliance initrd", flag.ContinueOnError)

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: initrdCommand,
			Summary: "Download or build the installer initrd",
			Usage:   []string{initrdCommand},
			Examples: []string{
				initrdCommand,
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	path, err := resolveInitrd()
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "initrd ready: %s\n", path) //nolint:errcheck // CLI output
	return exitOK
}

func resolveInitrd() (string, error) {
	version := defaultInitrdVersion
	arch := os.Getenv("GOARCH")
	if arch == "" {
		arch = runtime.GOARCH
	}
	cached := initrdCachePath(version, arch)
	toolsDst := filepath.Join(initrdToolsDir, initrdFileName)

	if _, err := os.Stat(cached); err == nil {
		if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
		}
		return cached, nil
	}

	if baseURL := env.Get(initrdURLKey); baseURL != "" {
		var tb textbuf.Buffer
		artifactURL := tb.Str(baseURL).Byte('/').Str(version).Byte('/').Str(initrdFileName).String()
		checksumURL := tb.Reset().Str(artifactURL).Str(checksumSuffix).String()
		if err := downloadAndVerify(artifactURL, checksumURL, cached); err == nil {
			if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
				fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
			}
			return cached, nil
		} else {
			fmt.Fprintf(os.Stdout, "warning: download from %s failed: %v; falling back to local build\n", baseURL, err) //nolint:errcheck // CLI warning
		}
	}

	missing := checkInitrdBuildTools()
	if len(missing) > 0 {
		return "", fmt.Errorf("installer initrd not cached; missing build tools: %v (or set %s for remote download)", missing, initrdURLKey)
	}

	if err := initrdMakeBuildFn(cached); err != nil {
		return "", fmt.Errorf("initrd build: %w", err)
	}

	if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
		fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
	}

	return cached, nil
}

func checkInitrdBuildTools() []string {
	return nil
}

func defaultInitrdMakeBuild(destPath string) error {
	arch := os.Getenv("GOARCH")
	if arch == "" {
		arch = runtime.GOARCH
	}

	var tb textbuf.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "ze-initrd-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort temp cleanup

	initBin := filepath.Join(tmpDir, "init")
	fmt.Fprintf(os.Stdout, "cross-compiling ze-installer for %s\n", arch) //nolint:errcheck // CLI output
	tags := "ze_installer"
	if os.Getenv("ZE_INITRD_FAULT") == "1" {
		// R-6 fault-injection evidence build ONLY (used by the QEMU harness): adds
		// the goroutine-panic hook in internal/install/disk/fault_linux.go. Never
		// set in production; the shipping initrd packs with just ze_installer.
		tags += ",ze_installer_fault"
		fmt.Fprintln(os.Stdout, "ZE_INITRD_FAULT=1: adding ze_installer_fault tag (R-6 evidence build)") //nolint:errcheck // CLI output
	}
	goArch := tb.Str("GOARCH=").Str(arch).String()
	goCmd := exec.CommandContext(ctx, "go", "build", //nolint:gosec // args are compile-time constants + validated arch
		"-tags", tags,
		"-o", initBin,
		"./cmd/ze-installer",
	)
	goCmd.Env = append(os.Environ(), "GOOS=linux", goArch, "CGO_ENABLED=0")
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stdout
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("build ze-installer (%s): %w", arch, err)
	}

	initData, err := os.ReadFile(initBin) //nolint:gosec // just-built binary
	if err != nil {
		return fmt.Errorf("read init binary: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	fmt.Fprintf(os.Stdout, "packing initrd (%d bytes) to %s\n", len(initData), destPath) //nolint:errcheck // CLI output
	return writeInitrdPack(destPath, initData)
}

// writeInitrdPack writes initData as the sole `init` entry (mode 0100755) of a
// newc cpio stream through a gzip writer to destPath. Output is reproducible:
// the gzip header Name/ModTime and the cpio mtime are all zeroed. Split out from
// defaultInitrdMakeBuild so the packing is unit-testable without a real build
// (AC-11, TestWriteInitrdPack).
func writeInitrdPack(destPath string, initData []byte) (err error) {
	out, err := os.Create(destPath) //nolint:gosec // dest from installer logic
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	outClosed := false
	defer func() {
		if !outClosed {
			if cerr := out.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("gzip writer: %w", err)
	}
	gz.Header.Name = ""
	gz.Header.ModTime = time.Time{}

	writeNewcEntry(gz, "init", 0o100755, initData)
	writeNewcTrailer(gz)

	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}
	outClosed = true
	return out.Close()
}

func writeNewcEntry(w io.Writer, name string, mode uint32, data []byte) {
	nameBytes := append([]byte(name), 0)
	nameLen := uint32(len(nameBytes))
	var hdr [110]byte
	copy(hdr[:6], "070701")
	putHex8(hdr[6:], 1)                  // ino
	putHex8(hdr[14:], mode)              // mode
	putHex8(hdr[22:], 0)                 // uid
	putHex8(hdr[30:], 0)                 // gid
	putHex8(hdr[38:], 1)                 // nlink
	putHex8(hdr[46:], 0)                 // mtime (zero for reproducibility)
	putHex8(hdr[54:], uint32(len(data))) // filesize
	putHex8(hdr[62:], 0)                 // devmajor
	putHex8(hdr[70:], 0)                 // devminor
	putHex8(hdr[78:], 0)                 // rdevmajor
	putHex8(hdr[86:], 0)                 // rdevminor
	putHex8(hdr[94:], nameLen)           // namesize
	putHex8(hdr[102:], 0)                // check
	w.Write(hdr[:])                      //nolint:errcheck // streaming to gzip writer; errors surface on gz.Close
	w.Write(nameBytes)                   //nolint:errcheck // streaming to gzip writer
	writePad4(w, 110+int(nameLen))
	w.Write(data) //nolint:errcheck // streaming to gzip writer
	writePad4(w, len(data))
}

func writeNewcTrailer(w io.Writer) {
	writeNewcEntry(w, "TRAILER!!!", 0, nil)
}

func putHex8(dst []byte, v uint32) {
	const digits = "0123456789ABCDEF"
	for i := 7; i >= 0; i-- {
		dst[i] = digits[v&0xf]
		v >>= 4
	}
}

func writePad4(w io.Writer, pos int) {
	pad := (4 - (pos % 4)) % 4
	if pad > 0 {
		var zeros [3]byte
		w.Write(zeros[:pad]) //nolint:errcheck // streaming to gzip writer
	}
}
