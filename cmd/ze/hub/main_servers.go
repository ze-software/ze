// Design: docs/architecture/hub-architecture.md -- Web, LG, and SSH server startup
// Related: main.go -- orchestration calls these, main_reload.go -- reload logic
// Detail: service_web.go -- ze_web-gated web service factory and web-only helpers

package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	yangloader "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// serverDispatcherWithSurface creates a CommandDispatcher with fixed audit surface attribution.
func serverDispatcherWithSurface(s *pluginserver.Server, surface string) func(command, username, remoteAddr string) (string, error) {
	return func(input, username, remoteAddr string) (string, error) {
		d := s.Dispatcher()
		if d == nil {
			return "", errServerNotReady
		}
		ctx := &pluginserver.CommandContext{Server: s, Username: username, RemoteAddr: remoteAddr, Surface: surface}
		resp, err := d.Dispatch(ctx, input)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", nil
		}
		if resp.Error != "" {
			return "", errors.New(resp.Error)
		}
		if resp.Status == plugin.StatusError {
			return "", errors.New("unknown error")
		}
		if resp.Data == nil {
			return "", nil
		}
		b, jsonErr := json.Marshal(resp.Data)
		if jsonErr != nil {
			return "", fmt.Errorf("marshal response: %w", jsonErr)
		}
		return string(b), nil
	}
}

// serverCommandLister creates a CommandLister from the plugin server's dispatcher.
func serverCommandLister(s *pluginserver.Server) zemcp.CommandLister {
	var (
		metaOnce          sync.Once
		paramsByPath      map[string][]zemcp.ParamInfo
		taskSupportByPath map[string]string
		uiResourceByPath  map[string]yangloader.UIResourceEntry
	)

	initMeta := func() {
		metaOnce.Do(func() {
			loader, err := yangloader.DefaultLoader()
			if err != nil {
				return
			}
			paramsByPath = buildParamMap(loader)
			taskSupportByPath = buildTaskSupportMap(loader)
			uiResourceByPath = yangloader.PathToUIResource(loader)
		})
	}

	return func() []zemcp.CommandInfo {
		d := s.Dispatcher()
		if d == nil {
			return nil
		}

		initMeta()

		var infos []zemcp.CommandInfo
		for _, cmd := range d.Commands() {
			info := zemcp.CommandInfo{
				Name:        cmd.Name,
				Help:        cmd.Help,
				ReadOnly:    cmd.ReadOnly,
				Params:      paramsByPath[cmd.Name],
				TaskSupport: parseTaskSupportLevel(taskSupportByPath[cmd.Name]),
			}
			if ui, ok := lookupUIResource(cmd.Name, uiResourceByPath); ok {
				info.UIResource = &zemcp.UIResourceInfo{
					Path:        ui.Path,
					Permissions: ui.Permissions,
					CSP:         ui.CSP,
				}
			}
			infos = append(infos, info)
		}

		// Plugin-registered commands.
		for _, cmd := range d.Registry().All() {
			infos = append(infos, zemcp.CommandInfo{
				Name: cmd.Name,
				Help: cmd.Description,
			})
		}

		return infos
	}
}

// endpointsToAddrs converts a slice of config.ServerEndpoint into the
// "host:port" string slice that every multi-listener binder accepts.
func endpointsToAddrs(servers []zeconfig.ServerEndpoint) []string {
	out := make([]string, 0, len(servers))
	for _, ep := range servers {
		out = append(out, ep.Listen())
	}
	return out
}

// mergeAuthUsers returns the zefs power user(s) followed by the config-file
// users. When a config user has the same name as a zefs power user, the config
// entry takes precedence and the zefs entry is dropped; this lets operators
// override the built-in password via configuration. The result is a fresh
// slice (never aliases either input).
func mergeAuthUsers(zefsUsers, configUsers []authz.UserConfig) []authz.UserConfig {
	cfgNames := make(map[string]bool, len(configUsers))
	for _, u := range configUsers {
		cfgNames[u.Name] = true
	}
	out := make([]authz.UserConfig, 0, len(zefsUsers)+len(configUsers))
	for _, u := range zefsUsers {
		if !cfgNames[u.Name] {
			out = append(out, u)
		}
	}
	out = append(out, configUsers...)
	return out
}

// loadZefsUsers reads credentials from the zefs database (created by ze init).
func loadZefsUsers() ([]authz.UserConfig, error) {
	dir := env.Get("ze.config.dir")
	if dir == "" {
		dir = paths.DefaultConfigDir()
	}
	if dir == "" {
		return nil, errCannotResolveConfigDirectory
	}
	dbPath := filepath.Join(dir, "database.zefs")
	db, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer db.Close() //nolint:errcheck // read-only access
	return usersFromZefsDB(db)
}

// usersFromZefsDB reads the dedicated local power-user credentials from zefs.
// Missing or empty credentials return an error so the caller fails closed.
// When meta/instance/admin-disabled is "true", returns errAdminDisabledInZefs
// so the caller skips the built-in power user.
func usersFromZefsDB(db *zefs.BlobStore) ([]authz.UserConfig, error) {
	if disabled, err := db.ReadFile(zefs.KeyInstanceAdminDisabled.Pattern); err == nil && string(disabled) == "true" {
		return nil, errAdminDisabledInZefs
	}
	username, err := db.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read local username: %w", err)
	}
	hash, err := db.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
	if err != nil {
		return nil, fmt.Errorf("read local password hash: %w", err)
	}
	name, err := validateLocalAdminCreds(username, hash)
	if err != nil {
		return nil, err
	}
	return []authz.UserConfig{{Name: name, Hash: string(hash)}}, nil
}

func validateLocalAdminCreds(username, hash []byte) (string, error) {
	name := string(username)
	if name == "" {
		return "", errEmptyUsernameInZefs
	}
	if len(hash) == 0 {
		// Fail closed: never hand an empty password hash to the authorizer.
		return "", errEmptyPasswordInZefs
	}
	return name, nil
}

// resolveConfigPath returns the config file path for the editor.
func resolveConfigPath(store storage.Storage) string {
	data, err := store.ReadFile(zefs.KeyInstanceName.Pattern)
	if err == nil && len(data) > 0 {
		name := strings.TrimSpace(string(data))
		if name != "" {
			var tb textbuf.Buffer
			return tb.Str(name).Str(".conf").String()
		}
	}
	return "ze.conf"
}
