package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	for _, name := range []string{"auth-reject", scenarioCLIReadsMetaKeys, scenarioClientBackupStart, scenarioClientCachedBoot, scenarioClientFirstBoot, scenarioClientReconnect, "client-reject-invalid", scenarioConfigChangeNotify, "file-namespace", scenarioInitManagedKey, "init-meta-keys", "per-client-auth"} {
		Register("managed/"+name, func(ctx context.Context, args []string) error { return managedLegacyScenario(ctx, name, args) })
	}
}

const managedCachedBase = `bgp {
 peer p1 {
  connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; } }
  session { asn { local 65000; remote 65001; } router-id 1.2.3.4; }
 }
}
`

func managedLegacyScenario(ctx context.Context, scenario string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("managed/%s takes no arguments", scenario)
	}
	if err := os.WriteFile("daemon.ready", nil, 0o600); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "managed-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	db := filepath.Join(dir, "database.zefs")
	managed := scenario == scenarioClientBackupStart || scenario == scenarioClientCachedBoot || scenario == scenarioClientFirstBoot || scenario == scenarioClientReconnect || scenario == scenarioInitManagedKey
	name := ""
	switch scenario {
	case scenarioClientBackupStart, scenarioClientCachedBoot, scenarioClientFirstBoot, scenarioClientReconnect:
		name = "edge-01"
	case scenarioConfigChangeNotify:
		name = "hub-01"
	}
	host, port := addrLoopback, "2222"
	if scenario == scenarioCLIReadsMetaKeys {
		host, port = addrPeerOne, "3333"
	}
	input := "admin\nsecret123\n" + host + "\n" + port + "\n" + name + "\n"
	env := miscEnvironment(map[string]string{envConfigDir: dir})
	initArgs := []string{argInit}
	if managed {
		initArgs = append(initArgs, "--managed")
	}
	if _, err := commandOutput(ctx, "", env, input, "ze", initArgs...); err != nil {
		return err
	}
	cat := func(key string) (string, error) {
		out, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "cat", key)
		return strings.TrimSpace(string(out)), err
	}
	mustEqual := func(key, want string) error {
		got, err := cat(key)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%s=%q, want %q", key, got, want)
		}
		return nil
	}
	switch scenario {
	case "auth-reject":
		config := `plugin { hub { server central { ip 0.0.0.0; port 1791; secret "central-secret-that-is-at-least32"; client edge-01 { secret "short"; } } } }`
		if _, err := commandOutput(ctx, "", os.Environ(), config, "ze", "config", "validate", "-"); err == nil {
			return errors.New("expected validation failure for short client secret")
		}
		fmt.Fprintln(os.Stderr, "OK: short per-client secret correctly rejected")
	case scenarioCLIReadsMetaKeys:
		if err := mustEqual("meta/ssh/default", "10.0.0.1/3333"); err != nil {
			return err
		}
		if err := mustEqual("meta/ssh/10.0.0.1/3333/username", "admin"); err != nil {
			return err
		}
		password, err := cat("meta/ssh/10.0.0.1/3333/password")
		if err != nil {
			return err
		}
		if !strings.HasPrefix(password, "$2") {
			return errors.New("password is not a bcrypt hash")
		}
		fmt.Fprintln(os.Stderr, "OK: ze data reads meta/ssh/* keys")
	case scenarioClientBackupStart, scenarioClientCachedBoot, scenarioClientReconnect:
		if scenario == scenarioClientBackupStart {
			if err := mustEqual("meta/instance/managed", "true"); err != nil {
				return err
			}
		}
		config := managedCachedBase + `plugin { hub { server local { ip 127.0.0.1; port 1790; secret "local-secret-that-is-at-least-32ch"; }`
		if scenario != scenarioClientCachedBoot {
			config += ` client edge-01 { host 10.0.0.1; port 1791; secret "client-token-that-is-at-least-32ch"; }`
		}
		config += " } }\n"
		path := filepath.Join(dir, "edge-01.conf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			return err
		}
		if _, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "import", path); err != nil {
			return err
		}
		listing, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "ls")
		if err != nil || !strings.Contains(string(listing), "edge-01.conf") {
			return fmt.Errorf("cached config not found: %w", err)
		}
		if scenario != scenarioClientBackupStart {
			active, err := cat("file/active/edge-01.conf")
			if err != nil {
				return err
			}
			if _, err := commandOutput(ctx, "", os.Environ(), active, "ze", "config", "validate", "-"); err != nil {
				return err
			}
		}
		messages := map[string]string{scenarioClientBackupStart: "OK: cached config written for managed client", scenarioClientCachedBoot: "OK: cached config written and valid for managed boot", scenarioClientReconnect: "OK: config with hub client block valid for reconnect"}
		fmt.Fprintln(os.Stderr, messages[scenario])
	case scenarioClientFirstBoot:
		if err := mustEqual("meta/instance/managed", "true"); err != nil {
			return err
		}
		out, _, err := rawCommand(ctx, "", env, "ze", "start")
		if err != nil {
			return err
		}
		if !strings.Contains(out, "managed mode") || !strings.Contains(out, "ze.managed.server") {
			return fmt.Errorf("missing managed-mode hints: %s", out)
		}
		fmt.Fprintln(os.Stderr, "OK: first boot without hub info exits with clear error")
	case "client-reject-invalid":
		config := `plugin { hub { server local { ip 127.0.0.1; port notanumber; secret "local-secret-that-is-at-least-32ch"; } } }`
		if _, err := commandOutput(ctx, "", os.Environ(), config, "ze", "config", "validate", "-"); err == nil {
			return errors.New("expected validation failure for invalid port")
		}
		fmt.Fprintln(os.Stderr, "OK: invalid hub config correctly rejected")
	case scenarioConfigChangeNotify:
		config := managedValidationConfig(false)
		if _, err := commandOutput(ctx, "", os.Environ(), config, "ze", "config", "validate", "-"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: hub config with per-client entries parses for notification")
	case "file-namespace":
		path := filepath.Join(dir, "test-router.conf")
		if err := os.WriteFile(path, []byte("router-id 1.1.1.1\n"), 0o600); err != nil {
			return err
		}
		if _, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "import", path); err != nil {
			return err
		}
		out, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "ls")
		fmt.Fprint(os.Stderr, string(out))
		if err != nil || !strings.Contains(string(out), "file/active/") {
			return errors.New("no file/active key found")
		}
		fmt.Fprintln(os.Stderr, "OK: config stored under file/active/ namespace")
	case scenarioInitManagedKey:
		if err := mustEqual("meta/instance/managed", "true"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: ze init --managed sets meta/instance/managed=true")
	case "init-meta-keys":
		out, err := commandOutput(ctx, "", os.Environ(), "", "ze", "data", "--path", db, "ls", "meta/ssh")
		if err != nil {
			return err
		}
		text := string(out)
		fmt.Fprint(os.Stderr, text)
		for _, key := range []string{"meta/ssh/127.0.0.1/2222/username", "meta/ssh/127.0.0.1/2222/password", "meta/ssh/default"} {
			if !strings.Contains(text, key) {
				return fmt.Errorf("%s not found", key)
			}
		}
		if err := mustEqual("meta/ssh/default", "127.0.0.1/2222"); err != nil {
			return err
		}
		if err := mustEqual("meta/instance/managed", "false"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: ze init writes meta/ssh/* keys")
	case "per-client-auth":
		if _, err := commandOutput(ctx, "", os.Environ(), managedValidationConfig(true), "ze", "config", "validate", "-"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: per-client auth config parses correctly")
	default:
		return fmt.Errorf("unknown scenario %s", scenario)
	}
	return nil
}

func managedValidationConfig(twoServers bool) string {
	localServer := ""
	secondClient := ` client edge-02 { secret "edge02-secret-that-is-at-least-32"; }`
	if twoServers {
		localServer = `server local { ip 127.0.0.1; port 1790; secret "local-secret-that-is-at-least-32ch"; }`
		secondClient = ""
	}
	return managedCachedBase + `plugin { hub { ` + localServer + ` server central { ip 0.0.0.0; port 1791; secret "central-secret-that-is-at-least32"; client edge-01 { secret "edge01-secret-that-is-at-least-32"; }` + secondClient + ` } } }`
}
