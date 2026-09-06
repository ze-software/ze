// Design: plan/spec-mgmt-version-header-suppress.md -- the management-hardening toggle
// Related: register_version_header_suppress.go -- the scenario name
// Related: internal/core/version/version.go -- HTTPHeaderHidden, the toggle both servers read

package fixture

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// versionHeaderBcryptAdmin is a cost-4 bcrypt hash of "testpass". The web
// server needs a user in the configuration, and this scenario never
// authenticates: it reads the headers of the 401 the authentication middleware
// writes, which is the same addSecurityHeaders call an authenticated page gets.
const versionHeaderBcryptAdmin = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"

// versionHeaderReport writes one progress line to standard error, where the
// .ci test reads it as an expectation.
func versionHeaderReport(line string) {
	var tb textbuf.Buffer
	_ = tb.Str(line).Byte('\n').StdErr() //nolint:errcheck // a lost progress line still fails the test through its own assertion
}

// versionHeaderConfig writes the configuration for one daemon. hardening is
// either empty or the `hide-version true` line, so the two runs differ by that
// leaf alone.
func versionHeaderConfig(hardening string, webPort, lgPort int) string {
	var tb textbuf.Buffer
	tb.Str(`system {
	authentication {
		user webadmin {
			password "`).Str(versionHeaderBcryptAdmin).Str(`"
			profile [ admin ]
		}
	}
	authorization {
		profile admin {
			run { default-action allow }
			edit { default-action allow }
		}
	}
}

environment {
	`).Str(hardening).Str(`
	web {
		enabled true
		server main {
			ip 127.0.0.1;
			port `).Int(int64(webPort)).Str(`;
		}
	}
	looking-glass {
		enabled true
		# The banner, not the transport, is what this scenario reads, so the
		# looking glass serves plaintext and the probe needs no handshake.
		tls false
		server main {
			ip 127.0.0.1;
			port `).Int(int64(lgPort)).Str(`;
		}
	}
}
`)
	return tb.String()
}

// versionHeaderResponseHeaders sends one GET and answers the response headers.
// The web listener serves a generated self-signed certificate, so the client
// skips verification: what this scenario reads is the header set, not the chain.
func versionHeaderResponseHeaders(ctx context.Context, url string) (http.Header, error) {
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // the fixture reads headers off a self-signed test listener
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer response.Body.Close() //nolint:errcheck // the body is drained and dropped
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return nil, fmt.Errorf("GET %s: read body: %w", url, err)
	}
	return response.Header, nil
}

// versionHeaderRun starts one daemon on the given configuration, waits for both
// HTTP servers to announce their listener, and answers the headers of one
// response from each. The daemon is stopped before it returns.
func versionHeaderRun(ctx context.Context, root, name, config string, webPort, lgPort int, environment []string) (http.Header, http.Header, error) {
	path := filepath.Join(root, name+".conf")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(root, name+".log")
	daemon, err := startProcess09(ctx, []string{actionStart, path}, nil, environment, logPath)
	if err != nil {
		return nil, nil, fmt.Errorf("start the %s daemon: %w", name, err)
	}
	defer stopProcess09(daemon)

	webBanner := "web server listening on https://127.0.0.1:" + strconv.Itoa(webPort) + "/"
	if logText, waitErr := waitForLog09(ctx, logPath, webBanner); waitErr != nil {
		return nil, nil, fmt.Errorf("%s run: %w: %s", name, waitErr, logText)
	}
	lgBanner := "looking glass listening on http://127.0.0.1:" + strconv.Itoa(lgPort) + "/"
	if logText, waitErr := waitForLog09(ctx, logPath, lgBanner); waitErr != nil {
		return nil, nil, fmt.Errorf("%s run: %w: %s", name, waitErr, logText)
	}

	web, err := versionHeaderResponseHeaders(ctx, "https://127.0.0.1:"+strconv.Itoa(webPort)+"/show/")
	if err != nil {
		return nil, nil, fmt.Errorf("%s run, web server: %w", name, err)
	}
	lg, err := versionHeaderResponseHeaders(ctx, "http://127.0.0.1:"+strconv.Itoa(lgPort)+"/api/looking-glass/status")
	if err != nil {
		return nil, nil, fmt.Errorf("%s run, looking glass: %w", name, err)
	}
	return web, lg, nil
}

// versionHeaderNoServerBanner reports the failure of the standing claim that Ze
// sends no Server header. Go's net/http omits it and Ze sets none, so a value
// here means a new writer put the build back on the wire under another name.
func versionHeaderNoServerBanner(server string, headers http.Header) error {
	if got := headers.Get("Server"); got != "" {
		return fmt.Errorf("the %s sent Server: %q, want no Server banner", server, got)
	}
	return nil
}

// versionHeaderSurvivors reports the failure of any header the toggle must
// leave alone. Suppression covers the version banner and nothing else.
func versionHeaderSurvivors(server string, headers http.Header, want map[string]string) error {
	for name, value := range want {
		if got := headers.Get(name); got != value {
			return fmt.Errorf("the %s sent %s: %q, want %q", server, name, got, value)
		}
	}
	return nil
}

// versionHeaderSuppress proves that one `hide-version` leaf takes the
// X-Ze-Version build banner off the web interface and the looking glass
// together, and that a configuration which does not write it keeps the banner.
//
// The two runs share one configuration text apart from that leaf, so the leaf
// is the only thing a difference in the headers can be attributed to.
func versionHeaderSuppress(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-version-header-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	adminDir := filepath.Join(root, "admin")
	if err := os.Mkdir(adminDir, 0o700); err != nil {
		return err
	}
	webPort, err := availablePort09()
	if err != nil {
		return err
	}
	lgPort, err := availablePort09()
	if err != nil {
		return err
	}
	sshPort, err := availablePort09()
	if err != nil {
		return err
	}

	var answers textbuf.Buffer
	answers.Str("admin\ntestpass\n127.0.0.1\n").Int(int64(sshPort)).Byte('\n')

	environment := append(os.Environ(), "ZE_CONFIG_DIR="+adminDir, "ZE_STORAGE_BLOB=true")
	initCommand := exec.CommandContext(ctx, "ze", "init")
	initCommand.Env = environment
	initCommand.Stdin = strings.NewReader(answers.String())
	initCommand.Stdout = io.Discard
	initCommand.Stderr = os.Stderr
	if err := initCommand.Run(); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}

	web, lg, err := versionHeaderRun(ctx, root, "default", versionHeaderConfig("", webPort, lgPort), webPort, lgPort, environment)
	if err != nil {
		return err
	}
	if banner := web.Get("X-Ze-Version"); !strings.Contains(banner, "ze/") {
		return fmt.Errorf("the default web server sent X-Ze-Version: %q, want the ze/ banner", banner)
	}
	if banner := lg.Get("X-Ze-Version"); !strings.Contains(banner, "ze/") {
		return fmt.Errorf("the default looking glass sent X-Ze-Version: %q, want the ze/ banner", banner)
	}
	if err := versionHeaderNoServerBanner("default web server", web); err != nil {
		return err
	}
	if err := versionHeaderNoServerBanner("default looking glass", lg); err != nil {
		return err
	}
	versionHeaderReport("OK: both servers send the version banner by default")

	web, lg, err = versionHeaderRun(ctx, root, "hidden", versionHeaderConfig("hide-version true", webPort, lgPort), webPort, lgPort, environment)
	if err != nil {
		return err
	}
	if banner := web.Get("X-Ze-Version"); banner != "" {
		return fmt.Errorf("the hardened web server sent X-Ze-Version: %q, want no banner", banner)
	}
	if banner := lg.Get("X-Ze-Version"); banner != "" {
		return fmt.Errorf("the hardened looking glass sent X-Ze-Version: %q, want no banner", banner)
	}
	versionHeaderReport("OK: hide-version takes the banner off both servers")

	if err := versionHeaderSurvivors("hardened web server", web, map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Cache-Control":             "no-store",
	}); err != nil {
		return err
	}
	if err := versionHeaderSurvivors("hardened looking glass", lg, map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	}); err != nil {
		return err
	}
	versionHeaderReport("OK: the other security headers survive suppression")

	if err := versionHeaderNoServerBanner("hardened web server", web); err != nil {
		return err
	}
	if err := versionHeaderNoServerBanner("hardened looking glass", lg); err != nil {
		return err
	}
	versionHeaderReport("OK: neither server sends a Server banner")
	return nil
}
