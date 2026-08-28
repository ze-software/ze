// Design: docs/guide/developer-setup.md -- local web server test workflow.
package setup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const webTestAddress = "3443"

// webTestReport records the isolated config directory and server status.
type webTestReport struct {
	ConfigDir string `json:"config-dir"`
	Address   string `json:"address"`
	Code      int    `json:"code"`
}

func runWebTestServer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	configDir, err := os.MkdirTemp(filepath.Join(root, "tmp"), "web-test.")
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	environ := append(os.Environ(), "ZE_CONFIG_DIR="+configDir)
	launcher := filepath.Join(root, "ze")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	initCommand := exec.CommandContext(ctx, launcher, "init", "--web-cert", "127.0.0.1:"+webTestAddress) //nolint:gosec // fixed checkout launcher and arguments
	initCommand.Dir = root
	initCommand.Env = environ
	initCommand.Stdin = strings.NewReader("admin\nsecret\n127.0.0.1\n2222\nweb-test\n")
	initCommand.Stdout = os.Stdout
	initCommand.Stderr = os.Stderr
	if err := initCommand.Run(); err != nil {
		code := processExitCode(err)
		return webTestReport{ConfigDir: configDir, Address: webTestAddress, Code: code}, code
	}

	startCommand := exec.CommandContext(ctx, launcher, "start", "--web", webTestAddress, "--insecure-web") //nolint:gosec // fixed checkout launcher and arguments
	startCommand.Dir = root
	startCommand.Env = environ
	startCommand.Stdin = os.Stdin
	startCommand.Stdout = os.Stdout
	startCommand.Stderr = os.Stderr
	err = startCommand.Run()
	code := processExitCode(err)
	return webTestReport{ConfigDir: configDir, Address: webTestAddress, Code: code}, code
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if code := exit.ExitCode(); code >= 0 {
			return code
		}
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 2
}
