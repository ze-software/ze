// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-auth-local plugin lifecycle
// Related: l2tpauthlocal.go -- atomic logger, Name constant
// Related: auth.go -- localAuth handler

package l2tpauthlocal

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpauthlocal/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

var authInstance = newLocalAuth()

func init() {
	l2tp.RegisterAuthHandler(authInstance.handle)

	reg := registry.Registration{
		Name:                    Name,
		Description:             "Static local user list for L2TP PPP authentication",
		Features:                "yang",
		YANG:                    schema.ZeL2TPAuthLocalConfYANG,
		ConfigRoots:             []string{"l2tp"},
		InProcessConfigVerifier: verifyLocalAuthConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", Name, err)
		os.Exit(1)
	}
}

func verifyLocalAuthConfig(sections []sdk.ConfigSection) error {
	for _, sec := range sections {
		if sec.Root != "l2tp" {
			continue
		}
		if _, err := parseUsersFromJSON(sec.Data); err != nil {
			return err
		}
	}
	return nil
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigVerify(verifyLocalAuthConfig)

	var pending map[string]userEntry

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "l2tp" {
				continue
			}
			users, err := parseUsersFromJSON(sec.Data)
			if err != nil {
				return err
			}
			pending = users
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pending != nil {
			authInstance.setUsers(pending)
			logger().Info("l2tp-auth-local: loaded users", "count", len(pending))
			pending = nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pending = nil
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"l2tp"},
		VerifyBudget: 1,
		ApplyBudget:  1,
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

// parseUsersFromJSON extracts the user list from the JSON config data.
// JSON shape: {"auth":{"local":{"user":[{"name":"x","password":"y"}]}}}.
func parseUsersFromJSON(data string) (map[string]userEntry, error) {
	if data == "" {
		return make(map[string]userEntry), nil
	}
	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return nil, fmt.Errorf("%s: invalid config JSON: %w", Name, err)
	}
	return parseUsersFromTree(tree)
}

func parseUsersFromTree(tree map[string]any) (map[string]userEntry, error) {
	users := make(map[string]userEntry)

	authBlock, ok := tree["auth"].(map[string]any)
	if !ok {
		return users, nil
	}
	localBlock, ok := authBlock["local"].(map[string]any)
	if !ok {
		return users, nil
	}
	userList, ok := localBlock["user"]
	if !ok {
		return users, nil
	}

	entries, ok := userList.([]any)
	if !ok {
		if single, ok2 := userList.(map[string]any); ok2 {
			entries = []any{single}
		} else {
			return nil, fmt.Errorf("%s: invalid user list type %T", Name, userList)
		}
	}

	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("%s: user entry missing name", Name)
		}
		pw, _ := m["password"].(string)
		users[name] = userEntry{Name: name, secret: pw}
	}
	return users, nil
}

// SetUsersForTest allows tests to inject a user table without config.
func SetUsersForTest(entries map[string]string) {
	users := make(map[string]userEntry, len(entries))
	for name, pw := range entries {
		users[name] = userEntry{Name: name, secret: pw}
	}
	authInstance.setUsers(users)
}

// ResetForTest clears the user table and is intended for use in test cleanup.
func ResetForTest() {
	authInstance.setUsers(make(map[string]userEntry))
}

// HandleForTest returns the auth handler function for direct testing.
func HandleForTest() func(req any) (bool, string) {
	return func(req any) (bool, string) {
		// Type assertion handled by caller
		return false, "use authInstance.handle directly"
	}
}

// OnConfigureForTest triggers config parsing from a tree for testing.
func OnConfigureForTest(tree map[string]any) error {
	users, err := parseUsersFromTree(tree)
	if err != nil {
		return err
	}
	authInstance.setUsers(users)
	return nil
}
