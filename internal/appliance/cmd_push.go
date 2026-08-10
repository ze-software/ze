// Design: docs/architecture/appliance/ota-push.md -- OTA push via vendored gokrazy updater

package appliance

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gokrazy/updater"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoImagesFoundRunzeAppliance = errors.New("no images found; run `ze appliance build <name>` first")

var doPushFn = doPushUpdater

func init() {
	cmdPush = runPush
}

func runPush(args []string) int {
	fs := flag.NewFlagSet("appliance push", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Push to all appliances with device.address set")
	imageFlag := fs.String("image", "", "Specific image file to push (default: most recent)")
	parallelFlag := fs.Int("parallel", 1, "Number of concurrent pushes (with --all)")
	testbootFlag := fs.Bool("testboot", false, "Mark as test boot (auto-revert on failure)")
	noRebootFlag := fs.Bool("no-reboot", false, "Stream and switch but do not reboot")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance push [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --image ze-20260427-143022.img lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --testboot lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --all\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --all --parallel 4\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	opts := pushOpts{
		testboot: *testbootFlag,
		noReboot: *noRebootFlag,
	}

	if *allFlag {
		return pushAll(*imageFlag, *parallelFlag, opts)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return pushOne(fs.Arg(0), *imageFlag, opts)
}

type pushOpts struct {
	testboot bool
	noReboot bool
}

func pushOne(name, imageFile string, opts pushOpts) int {
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if cfg.Device.Address == "" {
		fmt.Fprintf(os.Stderr, "error: device %s has no address configured\n", name)
		return exitError
	}

	var passphrase []byte
	if isEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	tokenPath := secretFilePath(dir, name, "update.token")
	updateToken, err := readSecret(tokenPath, passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read update token: %v\n", err)
		return exitError
	}
	defer ZeroBytes(updateToken)

	imgPath, err := resolveImagePath(dir, name, imageFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	tlsConfig, err := loadDeviceTLS(dir, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	port := cfg.Device.UpdatePort
	if port == 0 {
		port = 443
	}

	hostPort := net.JoinHostPort(cfg.Device.Address, strconv.Itoa(port))
	var tb textbuf.Buffer
	baseURL := tb.Str("https://").Str(hostPort).Byte('/').String()

	// Open the image once and use the same handle for checksum verification and
	// streaming, so the verified bytes are exactly the bytes pushed (no
	// re-open-by-path window for the file to change in between).
	imgFile, err := os.Open(imgPath) //nolint:gosec // path validated by resolveImagePath
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open image: %v\n", err)
		return exitError
	}
	defer imgFile.Close() //nolint:errcheck // read-only

	if err := verifyImageChecksum(imgFile, imgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if err := doPushFn(baseURL, imgFile, string(updateToken), tlsConfig, opts); err != nil {
		if isProtocolError(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: device %s unreachable at %s:%d\n", name, cfg.Device.Address, port)
		}
		return exitError
	}

	fmt.Fprintf(os.Stdout, "pushed %s to %s (%s:%d)\n", filepath.Base(imgPath), name, cfg.Device.Address, port) //nolint:errcheck // CLI output
	return exitOK
}

func pushAll(imageFile string, parallel int, opts pushOpts) int {
	names, code := listAddressedAppliances()
	if code != exitOK {
		return code
	}

	return runParallel(names, parallel, func(name string) int {
		fmt.Fprintf(os.Stderr, "pushing %s...\n", name)
		return pushOne(name, imageFile, opts)
	})
}

func listAddressedAppliances() ([]string, int) {
	dir := getBaseDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", dir, err)
		return nil, exitError
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == sharedDirName || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		cfg, loadErr := LoadConfig(ConfigPath(dir, e.Name()))
		if loadErr != nil {
			continue
		}
		if cfg.Device.Address != "" {
			names = append(names, e.Name())
		}
	}

	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "no appliances with device.address found in %s\n", dir)
		return nil, exitError
	}
	return names, exitOK
}

// verifyImageChecksum hashes the already-open image file f and compares it to
// the recorded .sha256 sidecar. It takes the open handle (rather than a path)
// so the bytes verified are the same bytes the caller streams to the device:
// re-opening by path would leave a window for the file to be replaced between
// verification and streaming.
func verifyImageChecksum(f *os.File, imgPath string) error {
	var tbChk textbuf.Buffer
	checksumPath := tbChk.Str(imgPath).Str(".sha256").String()
	checksumData, err := os.ReadFile(checksumPath) //nolint:gosec // appliance file
	if err != nil {
		// No .sha256 sidecar: checksum verification is optional, so skip it.
		return nil //nolint:nilerr // absent sidecar means "nothing to verify"
	}

	fields := strings.Fields(string(checksumData))
	if len(fields) == 0 {
		return nil
	}
	expectedHex := fields[0]

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek image for checksum: %w", err)
	}
	h := sha256.New()
	if _, copyErr := io.Copy(h, f); copyErr != nil {
		return fmt.Errorf("read image for checksum: %w", copyErr)
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != expectedHex {
		return fmt.Errorf("checksum mismatch for %s (expected %s..., got %s...)", filepath.Base(imgPath), expectedHex[:min(12, len(expectedHex))], actualHex[:12])
	}
	return nil
}

func resolveImagePath(baseDir, name, imageFile string) (string, error) {
	appDir := AppliancePath(baseDir, name)
	absAppDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", fmt.Errorf("resolve appliance dir: %w", err)
	}
	realAppDir, err := filepath.EvalSymlinks(absAppDir)
	if err != nil {
		return "", fmt.Errorf("resolve appliance dir: %w", err)
	}

	if imageFile != "" {
		path := filepath.Join(appDir, imageFile)
		return resolveContainedImagePath(realAppDir, path, imageFile)
	}

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", appDir, err)
	}

	var images []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "ze-") && strings.HasSuffix(e.Name(), ".img") {
			images = append(images, e.Name())
		}
	}

	if len(images) == 0 {
		return "", errNoImagesFoundRunzeAppliance
	}

	sort.Strings(images)
	selected := images[len(images)-1]
	return resolveContainedImagePath(realAppDir, filepath.Join(appDir, selected), selected)
}

func resolveContainedImagePath(realAppDir, path, displayName string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve image path: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("image %s not found", displayName)
	}
	rel, err := filepath.Rel(realAppDir, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("image path %s escapes appliance directory", displayName)
	}
	st, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("image %s not found", displayName)
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("image %s is not a regular file", displayName)
	}
	return realPath, nil
}

func loadDeviceTLS(baseDir, name string) (*tls.Config, error) {
	certPath := filepath.Join(tLSDir(baseDir, name), "cert.pem")
	certPEM, err := os.ReadFile(certPath) //nolint:gosec // appliance trust anchor
	if err != nil {
		return nil, fmt.Errorf("read device cert: %w", err)
	}

	pool := x509.NewCertPool()
	rest := certPEM
	added := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse device cert: %w", parseErr)
		}
		pool.AddCert(cert)
		added++
	}

	if added == 0 {
		return nil, fmt.Errorf("no certificates found in %s", certPath)
	}

	return &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}, nil
}

type protocolError struct{ msg string }

func (e *protocolError) Error() string { return e.msg }

func isProtocolError(err error) bool {
	var pe *protocolError
	return errors.As(err, &pe)
}

// authTransport injects HTTP basic auth on every request so that the updater
// library's internal requests (feature probe, stream, switch, reboot) all
// carry the update token.
type authTransport struct {
	base  http.RoundTripper
	token string
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// http.RoundTripper must not modify the request it receives. Clone before
	// injecting auth so a retried or redirected request keeps a clean original.
	req = req.Clone(req.Context())
	req.SetBasicAuth("", a.token)
	return a.base.RoundTrip(req) //nolint:wrapcheck // pass through
}

func doPushUpdater(baseURL string, img *os.File, updateToken string, tlsConfig *tls.Config, opts pushOpts) error {
	transport := &authTransport{
		base: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		token: updateToken,
	}
	client := &http.Client{Transport: transport}

	ctx := context.Background()

	target, err := updater.NewTarget(ctx, baseURL, client)
	if err != nil {
		return err
	}

	// The caller owns img and already verified its checksum from this handle;
	// rewind to the start before streaming.
	if _, err := img.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek image: %w", err)
	}

	fmt.Fprintf(os.Stderr, "streaming root image...\n")
	if err := target.StreamTo(ctx, "root", img); err != nil {
		return mapUpdaterError(err)
	}

	if opts.testboot {
		fmt.Fprintf(os.Stderr, "marking testboot...\n")
		if err := target.Testboot(ctx); err != nil {
			return mapUpdaterError(err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "switching active partition...\n")
		if err := target.Switch(ctx); err != nil {
			return mapUpdaterError(err)
		}
	}

	if !opts.noReboot {
		fmt.Fprintf(os.Stderr, "rebooting...\n")
		if err := target.Reboot(ctx); err != nil {
			return mapUpdaterError(err)
		}
	}

	return nil
}

func mapUpdaterError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") {
		return &protocolError{msg: "device rejected update (401 Unauthorized)"}
	}
	if strings.Contains(msg, "unexpected HTTP status") {
		return &protocolError{msg: msg}
	}
	return err
}
