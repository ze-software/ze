// Design: docs/guide/authentication.md -- a migrated account keeps its SHA-crypt hash
// Related: plugin_shell_extra_2.go -- the daemon start, SSH wait and login helpers this driver reuses

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The password both published SHA-crypt specification vectors in the config
// file were made from, a password one character different, and a piece of each
// vector's digest. The vectors are hashes another system wrote, so the
// login proves Ze verifies the format, not a hash it produced itself.
const (
	shaCryptAtBootPassword  = "Hello world!"
	shaCryptAtBootWrong     = "Hello world?"
	shaCryptAtBootDigest512 = "svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl"
	shaCryptAtBootDigest256 = "3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2"
)

func init() {
	Register("plugin/config-sha-crypt-password-at-boot", pluginSHACryptPasswordAtBoot)
}

// pluginSHACryptPasswordAtBoot starts the daemon on a config whose users carry
// a $6$ and a $5$ hash, logs each in over SSH with the right password, and
// checks a wrong one is refused.
func pluginSHACryptPasswordAtBoot(ctx context.Context, args []string) error {
	if err := pluginShellExtra2RequireNoArgs(args, "SHA-crypt password boot fixture"); err != nil {
		return err
	}
	daemon, err := pluginShellExtra2StartDaemon(ctx, "sha-crypt-at-boot.conf", map[string]string{
		envTestBGPPort: strconv.Itoa(10000 + os.Getpid()%50000),
	})
	if err != nil {
		return err
	}
	defer pluginShellExtra2StopDaemon(daemon)
	port, err := pluginShellExtra2WaitSSHPort(ctx)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, pluginShellExtra2Log(pluginShellExtra2DaemonLog))
	}
	emptyDir, err := os.MkdirTemp("", "sha-crypt-password-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(emptyDir) //nolint:errcheck // fixture cleanup

	login := func(user, password string) pluginShellExtra2Result {
		return pluginShellExtra2RunCombined(ctx, map[string]string{
			envConfigDir:   emptyDir,
			envSSHHost:     addrLoopback,
			envSSHPort:     port,
			envSSHPassword: password,
		}, "cli", "--user", user, "-c", "show bgp peer list")
	}

	var progress textbuf.Buffer
	for _, user := range []string{"migrated512", "migrated256"} {
		result := login(user, shaCryptAtBootPassword)
		if result.err != nil {
			return fmt.Errorf("%s must log in with the password its SHA-crypt hash was made from: %w\nOUTPUT: %s%s\n%s",
				user, result.err, result.stdout, result.stderr, pluginShellExtra2Log(pluginShellExtra2DaemonLog))
		}
		if !regexp.MustCompile(`SSH auth success.*username=` + user).MatchString(pluginShellExtra2Log(pluginShellExtra2DaemonLog)) {
			return fmt.Errorf("daemon did not log auth success for username=%s\n%s", user, pluginShellExtra2Log(pluginShellExtra2DaemonLog))
		}
		progress.Str("OK: ").Str(user).Str(" logged in with its SHA-crypt password\n")
	}

	refused := login("migrated512", shaCryptAtBootWrong)
	if refused.err == nil {
		return fmt.Errorf("a wrong password must not log migrated512 in\nOUTPUT: %s%s", refused.stdout, refused.stderr)
	}
	if !regexp.MustCompile(`SSH auth failure.*username=migrated512`).MatchString(pluginShellExtra2Log(pluginShellExtra2DaemonLog)) {
		return fmt.Errorf("daemon did not log the refused login for username=migrated512\n%s", pluginShellExtra2Log(pluginShellExtra2DaemonLog))
	}
	progress.Str("OK: a wrong password is refused\n")

	log := pluginShellExtra2Log(pluginShellExtra2DaemonLog)
	if strings.Contains(log, "not a valid bcrypt hash") {
		return fmt.Errorf("the daemon warned about a stored SHA-crypt hash\n%s", log)
	}
	if strings.Contains(log, shaCryptAtBootDigest512) {
		return errors.New("the stored $6$ hash reached the daemon log")
	}
	if strings.Contains(log, shaCryptAtBootDigest256) {
		return errors.New("the stored $5$ hash reached the daemon log")
	}
	if strings.Contains(log, shaCryptAtBootPassword) {
		return errors.New("a typed password reached the daemon log")
	}
	if strings.Contains(log, shaCryptAtBootWrong) {
		return errors.New("a typed wrong password reached the daemon log")
	}
	progress.Str("OK: no load warning, and neither hash nor password in the log\n").
		StdErr() //nolint:errcheck // fixture output; the .ci reads it
	return nil
}
