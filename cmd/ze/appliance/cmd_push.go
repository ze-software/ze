// Design: plan/spec-appliance-2-remote.md — OTA push via gokrazy HTTP update API

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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var errNoImagesFoundRunzeAppliance = errors.New("no images found; run `ze appliance build`")

func init() {
	cmdPush = runPush
}

func runPush(args []string) int {
	fs := flag.NewFlagSet("appliance push", flag.ContinueOnError)
	allFlag := fs.Bool("all", false, "Push to all appliances with device.address set")
	imageFlag := fs.String("image", "", "Specific image file to push (default: most recent)")
	parallelFlag := fs.Int("parallel", 1, "Number of concurrent pushes (with --all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance push [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --image ze-20260427-143022.img lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --all\n")
		fmt.Fprintf(os.Stderr, "  ze appliance push --all --parallel 4\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *allFlag {
		return pushAll(*imageFlag, *parallelFlag)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name> or --all\n")
		fs.Usage()
		return exitError
	}

	return pushOne(fs.Arg(0), *imageFlag)
}

func pushOne(name, imageFile string) int {
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
	if IsEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	tokenPath := secretFilePath(dir, name, "update.token")
	updateToken, err := ReadSecret(tokenPath, passphrase)
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
	endpoint := fmt.Sprintf("https://%s/update", hostPort)

	if err := verifyImageChecksum(imgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if err := doPush(endpoint, imgPath, string(updateToken), tlsConfig); err != nil {
		if isProtocolError(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: device %s unreachable at %s:%d\n", name, cfg.Device.Address, port)
		}
		return exitError
	}

	fmt.Printf("pushed %s to %s (%s:%d)\n", filepath.Base(imgPath), name, cfg.Device.Address, port)
	return exitOK
}

func pushAll(imageFile string, parallel int) int {
	names, code := listAddressedAppliances()
	if code != exitOK {
		return code
	}

	return runParallel(names, parallel, func(name string) int {
		fmt.Fprintf(os.Stderr, "pushing %s...\n", name)
		return pushOne(name, imageFile)
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

func verifyImageChecksum(imgPath string) error {
	checksumPath := imgPath + ".sha256"
	checksumData, err := os.ReadFile(checksumPath) //nolint:gosec // appliance file
	if err != nil {
		return nil
	}

	fields := strings.Fields(string(checksumData))
	if len(fields) == 0 {
		return nil
	}
	expectedHex := fields[0]

	f, err := os.Open(imgPath) //nolint:gosec // already validated path
	if err != nil {
		return fmt.Errorf("open image for checksum: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	h := sha256.New()
	buf := make([]byte, 256*1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != expectedHex {
		return fmt.Errorf("checksum mismatch for %s (expected %s..., got %s...)", filepath.Base(imgPath), expectedHex[:min(12, len(expectedHex))], actualHex[:12])
	}
	return nil
}

func resolveImagePath(baseDir, name, imageFile string) (string, error) {
	appDir := AppliancePath(baseDir, name)

	if imageFile != "" {
		path := filepath.Join(appDir, imageFile)
		resolved, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve image path: %w", err)
		}
		absAppDir, _ := filepath.Abs(appDir)
		if !strings.HasPrefix(resolved, absAppDir+string(filepath.Separator)) {
			return "", fmt.Errorf("image path %s escapes appliance directory", imageFile)
		}
		if _, err := os.Stat(resolved); err != nil {
			return "", fmt.Errorf("image %s not found", imageFile)
		}
		return resolved, nil
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
	return filepath.Join(appDir, images[len(images)-1]), nil
}

func loadDeviceTLS(baseDir, name string) (*tls.Config, error) {
	certPath := filepath.Join(TLSDir(baseDir, name), "cert.pem")
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
		MinVersion: tls.VersionTLS12,
	}, nil
}

type protocolError struct{ msg string }

func (e *protocolError) Error() string { return e.msg }

func isProtocolError(err error) bool {
	var pe *protocolError
	return errors.As(err, &pe)
}

func doPush(endpoint, imgPath, updateToken string, tlsConfig *tls.Config) error {
	f, err := os.Open(imgPath) //nolint:gosec // user-specified image path
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, endpoint, f)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth("", updateToken)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body

	if resp.StatusCode == http.StatusUnauthorized {
		return &protocolError{msg: "device rejected update (401 Unauthorized)"}
	}
	if resp.StatusCode != http.StatusOK {
		return &protocolError{msg: fmt.Sprintf("unexpected status: %s", resp.Status)}
	}

	return nil
}
