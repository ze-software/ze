// Design: docs/architecture/api/commands.md -- local-data commands served by the daemon too
// Related: ui_fixture_pipe_alias_completion.go -- the ssh terminal and Tab offer this reuses
// Related: plugin_fixture_11_alias.go -- startFixtureDaemon

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

func init() {
	registerFixture("ui/local-command-daemon-ssh", localCommandDaemonSSH)
}

// localCommandDaemonSSHConfig is a daemon with one SSH account and nothing
// else: the commands under test are local-data commands, which need no
// protocol and no plugin.
const localCommandDaemonSSHConfig = `system {
    authentication {
        user ci {
            password "$PASSWORD_HASH"
            profile [ admin ]
        }
    }
    authorization {
        profile admin { run { default-action allow; } edit { default-action allow; } }
    }
}
`

// localCommandDaemonSSH proves a local-data command reaches an SSH operator
// through the daemon, on both halves of the surface.
//
// Completion: the session model the daemon hosts offers `list` after
// `show env `, so `show env list` is discoverable over SSH, not only from
// `ze show env list` in the operator's own process.
//
// Execution: an SSH exec of `show config validation <file>` runs in the
// daemon. A file inside the daemon's configuration directory is read and
// judged valid; a file outside it is refused with the local command that
// reads any file, because the daemon route reads only inside its
// configuration directories (command.CallerPath).
func localCommandDaemonSSH(ctx context.Context, args []string) error {
	daemon, cliEnv, err := startFixtureDaemon(ctx, args, localCommandDaemonSSHConfig)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	account, err := localCommandLogin(cliEnv)
	if err != nil {
		return err
	}
	configDir, err := pipeAliasSetting(cliEnv, "ZE_CONFIG_DIR")
	if err != nil {
		return err
	}

	screen, err := pipeAliasTabOffer(ctx, account, "show env ", "env")
	if err != nil {
		return err
	}
	if !strings.Contains(screen, "list") {
		return fmt.Errorf("ssh completion after 'show env ' did not offer 'list':\n%s", screen)
	}

	creds := sshclient.Credentials{Host: "127.0.0.1", Port: account.port, Username: account.user, Auth: account.password}
	inside := filepath.Join(configDir, "ze.conf")
	answer, err := sshclient.ExecCommand(creds, "show config validation "+inside+" | json")
	if err != nil {
		return fmt.Errorf("ssh exec of show config validation on %s inside the config directory: %w", inside, err)
	}
	// `| json` indents a multi-line document, so the verdict is read from the
	// decoded answer rather than matched as text.
	var verdict struct {
		Valid *bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &verdict); err != nil {
		return fmt.Errorf("show config validation over ssh answered no JSON document for %s: %w\n%s", inside, err, answer)
	}
	if verdict.Valid == nil || !*verdict.Valid {
		return fmt.Errorf("show config validation over ssh did not judge %s valid:\n%s", inside, answer)
	}

	outsideDir, err := os.MkdirTemp("", "ze-outside-config-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(outsideDir) //nolint:errcheck // fixture teardown
	outside := filepath.Join(outsideDir, "ze.conf")
	if err := os.WriteFile(outside, []byte(localCommandDaemonSSHConfig), 0o600); err != nil {
		return err
	}
	refused, err := sshclient.ExecCommand(creds, "show config validation "+outside)
	if err == nil {
		return fmt.Errorf("show config validation over ssh read %s outside the config directory:\n%s", outside, refused)
	}
	if !strings.Contains(err.Error(), "run 'ze show config validation "+outside+"'") {
		return fmt.Errorf("the refusal of %s does not name the local command: %w", outside, err)
	}

	fmt.Println("OK: ssh offered show env list, validated", inside, "and refused", outside)
	return nil
}

// localCommandLogin answers the ssh account startFixtureDaemon created.
func localCommandLogin(cliEnv []string) (login, error) {
	port, err := pipeAliasSetting(cliEnv, "ZE_SSH_PORT")
	if err != nil {
		return login{}, err
	}
	user, err := pipeAliasSetting(cliEnv, "ZE_SSH_USERNAME")
	if err != nil {
		return login{}, err
	}
	password, err := pipeAliasSetting(cliEnv, "ZE_SSH_PASSWORD")
	if err != nil {
		return login{}, err
	}
	if port == "" {
		return login{}, errors.New("the fixture daemon published no ssh port")
	}
	return login{port: port, user: user, password: password}, nil
}
