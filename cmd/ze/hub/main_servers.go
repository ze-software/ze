// Design: docs/architecture/hub-architecture.md -- Web, LG, and SSH server startup
// Related: main.go -- orchestration calls these, main_reload.go -- reload logic

package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
	"codeberg.org/thomas-mangin/ze/internal/component/audit"
	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	bgpcli "codeberg.org/thomas-mangin/ze/internal/component/bgp/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	zeconfigcmd "codeberg.org/thomas-mangin/ze/internal/component/config/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	yangloader "codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	zegnmi "codeberg.org/thomas-mangin/ze/internal/component/gnmi"
	zegokrazy "codeberg.org/thomas-mangin/ze/internal/component/gokrazy"
	"codeberg.org/thomas-mangin/ze/internal/component/lg"
	zemcp "codeberg.org/thomas-mangin/ze/internal/component/mcp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// webOnlyDispatcher creates a minimal CommandDispatcher backed by a local event
// ring. Used by RunWebOnly where no plugin server exists.
func webOnlyDispatcher(ring *pluginserver.EventRing) zeweb.CommandDispatcher {
	return func(command, _, _ string) (string, error) {
		switch {
		case strings.HasPrefix(command, "show event namespaces"):
			counts := ring.NamespaceCounts()
			rows := make([]map[string]any, 0, len(counts))
			for ns, count := range counts {
				rows = append(rows, map[string]any{"namespace": ns, "count": count})
			}
			b, err := json.Marshal(map[string]any{"namespaces": rows})
			if err != nil {
				return "", err
			}
			return string(b), nil

		case strings.HasPrefix(command, "show event recent"):
			namespace := ""
			if _, after, ok := strings.Cut(command, "namespace "); ok {
				namespace = strings.TrimSpace(after)
			}
			records := ring.Snapshot(50, namespace)
			out := make([]map[string]any, 0, len(records))
			for i := range records {
				out = append(out, map[string]any{
					"timestamp":  records[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
					"namespace":  records[i].Namespace,
					"event-type": records[i].EventType,
				})
			}
			b, err := json.Marshal(map[string]any{"events": out, "count": len(out)})
			if err != nil {
				return "", err
			}
			return string(b), nil

		default:
			return "", errWebOnlyUnavailable
		}
	}
}

// errWebOnlyUnavailable is returned by webOnlyDispatcher for any operational
// command that needs the daemon. The message is written for direct display in
// the web UI (tools render dispatch errors inline; log pages map it to an
// honest empty state) rather than exposing the raw command string (F4/AC-10).
var errWebOnlyUnavailable = errors.New("operational commands require a running daemon with a loaded configuration; the web interface is running in standalone mode")

// withBGPDecode wraps a CommandDispatcher to handle "show bgp decode" in-process.
// BGP decode is a pure function registered as a local command; neither the
// plugin-server dispatcher nor the web-only stub knows about it. This wrapper
// intercepts the command and calls the decoder directly so the web tool page
// works in both full-daemon and web-only modes (F5/AC-8).
func withBGPDecode(inner zeweb.CommandDispatcher) zeweb.CommandDispatcher {
	const prefix = "show bgp decode "
	return func(command, username, remoteAddr string) (string, error) {
		if strings.HasPrefix(command, prefix) {
			hex := strings.TrimSpace(command[len(prefix):])
			return bgpcli.DecodeHexPacket(hex, "", "", false)
		}
		if inner != nil {
			return inner(command, username, remoteAddr)
		}
		return "", errWebOnlyUnavailable
	}
}

// wireEventRingToBroker connects an EventRing to an SSE broker so that new
// events are pushed to connected Live Log clients as "log-entry" SSE events.
func wireEventRingToBroker(ring *pluginserver.EventRing, broker *zeweb.EventBroker) {
	ring.SetOnAppend(func(rec pluginserver.EventRecord) {
		var tb textbuf.Buffer
		entry := tb.Str(rec.Timestamp.Format("15:04:05")).
			Byte(' ').Str(rec.Namespace).
			Byte(' ').Str(rec.EventType).String()
		broker.Broadcast("log-entry", entry)
	})
}

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

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server and SSE event broker on success, nil on failure (logged, non-fatal).
// Caller MUST call broker.Close() during shutdown to release SSE clients.
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
// Requires blob storage -- TLS keys and config must not leak to the filesystem.
func startWebServer(store storage.Storage, configPath string, listenAddrs []string, insecureWeb bool, dispatch zeweb.CommandDispatcher, resolvers *resolve.Resolvers, authorizer aaa.Authorizer, recorder audit.Recorder, commitHook func() error, configUsers []authz.UserConfig) (*zeweb.WebServer, *zeweb.EventBroker, *zeweb.EditorManager) {
	dispatch = withBGPDecode(dispatch)

	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: requires blob storage (run ze init first)\n")
		return nil, nil, nil
	}

	if len(listenAddrs) == 0 {
		listenAddrs = []string{"0.0.0.0:3443"}
	}

	var users []authz.UserConfig
	var powerUserNames []string
	if !insecureWeb {
		// Both the always-on zefs power user and config-file users may log in.
		// A failure to load the power user is not fatal as long as config users
		// exist; the server is only disabled when there are no users at all
		// (fail closed -- never serve an unauthenticated admin UI).
		if zefsUsers, err := loadZefsUsers(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: web power-user auth unavailable: %v\n", err)
		} else {
			users = zefsUsers
			for _, u := range zefsUsers {
				powerUserNames = append(powerUserNames, u.Name)
			}
		}
		users = mergeAuthUsers(users, configUsers)
		if len(users) == 0 {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: no authenticatable users\n")
			return nil, nil, nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
		if zefsUsers, err := loadZefsUsers(); err == nil {
			for _, u := range zefsUsers {
				powerUserNames = append(powerUserNames, u.Name)
			}
		}
	}

	// Persist TLS cert in zefs so browsers don't have to re-accept on every restart.
	// The SAN hint is derived from the first endpoint; GenerateWebCertWithAddr
	// already fans out to all interface IPs when the host is 0.0.0.0.
	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddrs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: TLS cert: %v\n", err)
		return nil, nil, nil
	}

	renderer, err := zeweb.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: renderer: %v\n", err)
		return nil, nil, nil
	}

	// Register display-time decorators (e.g., ASN -> org name via Team Cymru DNS).
	decorators := zeweb.NewDecoratorRegistry()
	if resolvers != nil && resolvers.Cymru != nil {
		decorators.Register(zeweb.NewASNNameDecoratorFromCymru(resolvers.Cymru))
	}
	renderer.SetDecorators(decorators)

	srv, err := zeweb.NewWebServer(zeweb.WebConfig{
		ListenAddrs: listenAddrs,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
		return nil, nil, nil
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: YANG schema: %v\n", schemaErr)
		return nil, nil, nil
	}

	var commandTree *command.Node
	// Strict ze:related validation against the full operational command
	// tree. Surfaces typos and renamed-command drift at hub startup so
	// operators see the diagnostic before any workbench click. Logged as
	// a warning (not fatal) so a single drifted descriptor never prevents
	// the hub from serving the rest of the UI.
	if loader, loaderErr := yangloader.DefaultLoader(); loaderErr == nil {
		commandTree = yangloader.BuildCommandTree(loader)
		if validateErr := zeconfig.ValidateSchemaAgainstCommandTree(schema, commandTree); validateErr != nil {
			fmt.Fprintf(os.Stderr, "warning: ze:related validation: %v\n", validateErr)
		}
	} else {
		fmt.Fprintf(os.Stderr, "warning: operational command tree unavailable: %v\n", loaderErr)
	}

	// Ensure a config file exists for the editor.
	if configPath == "" {
		configPath = resolveConfigPath(store)
	}
	if !store.Exists(configPath) {
		if writeErr := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create config: %v\n", writeErr)
		}
	}

	// Parse the committed config into the live baseline tree used by compare
	// targets. Saved drafts are a separate source, not the startup-loaded config.
	tree := zeconfig.NewTree()
	if configData, readErr := store.ReadFile(configPath); readErr == nil {
		if parsed, parseErr := zeconfig.ParseTreeWithYANG(string(configData), nil); parseErr == nil {
			tree = parsed
		}
	}

	// Create editor manager for config editing via web.
	editorMgr := zeweb.NewEditorManager(store, configPath, schema, newEditorFactory(zeconfigcmd.ValidateContent), newEditSessionFactory())
	if commitHook != nil {
		// Install before serving so early commits cannot bypass daemon reload.
		editorMgr.SetCommitHook(commitHook)
	}

	var commandCompleter zeweb.CommandCompleter
	if commandTree != nil {
		commandCompleter = cli.NewCommandCompleter(commandTree)
	}
	// Create CLI completer for Tab/? autocomplete.
	completer := cli.NewCompleter()

	sessionStore := zeweb.NewSessionStore()
	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		if renderErr := renderer.RenderLogin(w, zeweb.LoginData{ReturnTo: r.URL.RequestURI()}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	// SSE broker for live config change notifications and log streaming.
	broker := zeweb.NewEventBroker(0)

	// Workbench is the normal UI. Finder remains as a server-side rollback when
	// ze.web.ui=finder is set before startup; stale browser cookies do not switch
	// operators back to the deprecated shell.
	uiMode := zeweb.GetUIMode()
	finderHandler := zeweb.HandleFragment(renderer, schema, tree, editorMgr, insecureWeb)
	workbenchHandler := zeweb.HandleWorkbench(renderer, schema, tree, editorMgr, insecureWeb,
		zeweb.WithDispatch(dispatch), zeweb.WithBroker(broker), zeweb.WithPowerUsers(powerUserNames))
	fmt.Fprintf(os.Stderr, "web UI default: %s\n", uiMode)
	showHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch zeweb.ReadUIModeFromRequest(r, uiMode) {
		case zeweb.UIModeWorkbench:
			workbenchHandler(w, r)
		default:
			finderHandler(w, r)
		}
	})
	// Fragment handler still serves /fragment/detail HTMX requests regardless
	// of mode; both UIs share the same OOB swap path.
	fragmentHandler := finderHandler

	// Config set, add, and delete handlers for editing leaf values.
	setHandler := zeweb.HandleConfigSetWithAuthorizer(editorMgr, schema, renderer, authorizer)
	formHandler := zeweb.HandleConfigFormWithAuthorizer(editorMgr, schema, renderer, authorizer)
	addHandler := zeweb.HandleConfigAddWithAuthorizer(editorMgr, schema, renderer, authorizer)
	addFormHandler := zeweb.HandleConfigAddForm(editorMgr, schema, renderer)
	renameHandler := zeweb.HandleConfigRenameWithAuthorizer(editorMgr, schema, authorizer)
	deleteHandler := zeweb.HandleConfigDeleteWithAuthorizer(editorMgr, authorizer)

	// Commit and discard handlers.
	commitHandler := zeweb.HandleConfigCommitWithAuthorizerAndAudit(editorMgr, renderer, broker, authorizer, recorder)
	discardHandler := zeweb.HandleConfigDiscardWithAuthorizerAndAudit(editorMgr, authorizer, recorder)

	// Diff handler: returns the diff modal HTML (open, with content).
	diffHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := zeweb.GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		diff, _ := editorMgr.Diff(username)
		count := editorMgr.ChangeCount(username)
		type diffData struct {
			Diff        string
			ChangeCount int
		}
		html := renderer.RenderFragment("diff_modal_open", diffData{Diff: diff, ChangeCount: count})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// Diff close: returns the closed modal HTML.
	diffCloseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html := renderer.RenderFragment("diff_modal", nil)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// CLI handlers: command execution, autocomplete, terminal mode.
	cliHandler := zeweb.HandleCLICommandWithAuthorizer(editorMgr, schema, renderer, authorizer)
	terminalHandler := zeweb.HandleCLITerminalWithDispatchAuthorizerAndAudit(editorMgr, schema, tree, dispatch, authorizer, recorder)
	modeHandler := zeweb.HandleCLIModeToggle(editorMgr, schema, renderer)
	completeHandler := zeweb.HandleCLICompleteWithCommandCompleter(completer, commandCompleter, editorMgr, schema)

	// Auth wrapper for protecting individual routes.
	webAuth := &authz.LocalAuthenticator{Users: users}
	var authWrap func(http.Handler) http.Handler
	if insecureWeb {
		authWrap = zeweb.InsecureMiddleware
	} else {
		authWrap = func(h http.Handler) http.Handler {
			return zeweb.AuthMiddlewareWithAudit(sessionStore, webAuth, loginRenderer, h, recorder)
		}
	}
	mutationWrap := func(h http.Handler) http.Handler {
		return authWrap(zeweb.RequireSameOrigin(h))
	}

	loginHandler := zeweb.LoginHandlerWithAudit(sessionStore, webAuth, loginRenderer, recorder)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	// Admin command tree for web UI. Derive from the merged YANG command
	// tree so plugin-contributed commands appear in the admin nav without
	// editing the static map (spec-web-2 Phase 6 / Spec D6). The static
	// fallback was removed because its tree shape (`peer/route/cache`)
	// drifted from the YANG-derived shape (`peer/show/summary/...`); a
	// silent fallback after loader failure would surface as broken admin
	// links rather than a clear error.
	var adminChildren map[string][]string
	if commandTree != nil {
		adminChildren = zeweb.AdminTreeFromYANG(commandTree)
	} else {
		adminChildren = map[string][]string{}
	}
	adminViewHandler := zeweb.HandleAdminView(renderer, adminChildren)
	adminExecHandler := zeweb.HandleAdminExecute(renderer, dispatch)

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	// Serve /favicon.ico from assets so the browser's automatic request does not
	// fall through to the catch-all and trigger an error-redirect (F14).
	srv.Handle("GET /favicon.ico", renderer.FaviconHandler())
	srv.Handle("/events", authWrap(broker))
	srv.Handle("/admin/", mutationWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			adminExecHandler(w, r)
			return
		}
		adminViewHandler(w, r)
	})))
	srv.Handle("GET /cli", authWrap(zeweb.HandleCLIPageHTTP(renderer, insecureWeb)))
	srv.Handle("POST /cli", mutationWrap(cliHandler))
	srv.Handle("/cli/complete", authWrap(completeHandler))
	srv.Handle("POST /cli/terminal", mutationWrap(terminalHandler))
	srv.Handle("POST /cli/mode", mutationWrap(modeHandler))
	srv.Handle("/fragment/detail", authWrap(fragmentHandler))
	srv.Handle("POST /config/set/", mutationWrap(setHandler))
	srv.Handle("POST /config/form/", mutationWrap(formHandler))
	srv.Handle("POST /config/add/", mutationWrap(addHandler))
	srv.Handle("GET /config/add-form/", authWrap(addFormHandler))
	srv.Handle("POST /config/rename/", mutationWrap(renameHandler))
	srv.Handle("GET /config/changes", authWrap(zeweb.HandleConfigChanges(editorMgr, renderer)))
	srv.Handle("POST /config/delete/", mutationWrap(deleteHandler))
	srv.Handle("/config/diff", authWrap(diffHandler))
	srv.Handle("/config/diff-close", authWrap(diffCloseHandler))
	srv.Handle("/config/commit", mutationWrap(commitHandler))
	srv.Handle("/config/commit/", mutationWrap(commitHandler))
	srv.Handle("POST /config/discard", mutationWrap(discardHandler))
	srv.Handle("POST /config/discard/", mutationWrap(discardHandler))
	// V2 workbench related-tool execution. Browser submits only tool id +
	// context path; the handler resolves the descriptor server-side and
	// dispatches via the standard CommandDispatcher (same authz pipeline
	// as /cli and /admin).
	srv.Handle("POST /tools/related/run", mutationWrap(zeweb.HandleRelatedToolRun(renderer, schema, tree, editorMgr, dispatch)))
	// L2TP web UI: session list, detail, CQM chart feeds, disconnect.
	l2tpHandlers := &zeweb.L2TPHandlers{Renderer: renderer, Dispatch: dispatch}
	srv.Handle("GET /l2tp", authWrap(l2tpHandlers.HandleL2TPList()))
	srv.Handle("GET /l2tp/{sid}", authWrap(l2tpHandlers.HandleL2TPDetail()))
	srv.Handle("POST /l2tp/{sid}/disconnect", mutationWrap(l2tpHandlers.HandleL2TPDisconnect()))
	srv.Handle("GET /l2tp/{login}/samples", authWrap(zeweb.HandleL2TPSamplesJSON()))
	srv.Handle("GET /l2tp/{login}/samples.csv", authWrap(zeweb.HandleL2TPSamplesCSV()))
	srv.Handle("GET /l2tp/{login}/samples/stream", authWrap(zeweb.HandleL2TPSamplesSSE()))

	// Portal: iframe wrapper for embedded services (gokrazy, etc.).
	if env.IsEnabled("ze.gokrazy.enabled") {
		srv.Handle("/gokrazy/", authWrap(zegokrazy.Handler(env.Get("ze.gokrazy.socket"))))
		zeweb.RegisterPortalService(zeweb.PortalService{
			Key: "gokrazy", Title: "Gokrazy", Path: "/gokrazy/",
			Icon: "/gokrazy/assets/gokrazy-logo.svg",
		})
	}
	srv.Handle("/portal/", authWrap(zeweb.HandlePortal(renderer, uiMode)))
	srv.Handle("GET /health", authWrap(health.DefaultRegistry.Handler()))
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}
		// /show and /monitor go through the active UI handler; everything else
		// (e.g. /fragment/detail HTMX requests) keeps using the Finder fragment
		// handler so the OOB swap protocol is identical across both UIs.
		if strings.HasPrefix(r.URL.Path, "/show/") || strings.HasPrefix(r.URL.Path, "/monitor/") {
			authWrap(showHandler).ServeHTTP(w, r)
			return
		}
		authWrap(fragmentHandler).ServeHTTP(w, r)
	})

	go func() {
		if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slogutil.Logger("web.server").Error("web server error", "error", serveErr)
		}
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if waitErr := srv.WaitReady(readyCtx); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server failed to start: %v\n", waitErr)
		_ = srv.Shutdown(context.Background())
		return nil, nil, nil
	}

	fmt.Fprintf(os.Stderr, "web server listening on https://%s/\n", srv.Address())
	return srv, broker, editorMgr
}

// mergeAuthUsers returns the always-on zefs power user(s) followed by the
// config-file users, so both authenticate on a surface. Mirrors the SSH paths
// (infra_setup.go / main.go). Order puts the power user first; the result is a
// fresh slice (never aliases either input).
//
// Duplicate names are intentionally NOT deduplicated: if a name appears in both
// sources (e.g. both define "admin"), both entries are kept and the
// authenticator accepts either password. There is no override -- the power user
// cannot be shadowed or disabled by a config-file user of the same name.
func mergeAuthUsers(zefsUsers, configUsers []authz.UserConfig) []authz.UserConfig {
	out := make([]authz.UserConfig, 0, len(zefsUsers)+len(configUsers))
	out = append(out, zefsUsers...)
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
func usersFromZefsDB(db *zefs.BlobStore) ([]authz.UserConfig, error) {
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

// blobCertStore implements web.CertStore backed by zefs blob storage.
type blobCertStore struct {
	store storage.Storage
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

// startLGServer creates and starts the looking glass HTTP server.
// Returns the server on success, nil on failure (logged, non-fatal).
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
func startLGServer(store storage.Storage, listenAddrs []string, useTLS bool, dispatch lg.CommandDispatcher, resolvers *resolve.Resolvers) *lg.LGServer {
	if len(listenAddrs) == 0 {
		return nil
	}
	cfg := lg.LGConfig{
		ListenAddrs: listenAddrs,
		TLS:         useTLS,
		Dispatch:    dispatch,
		DecorateASN: func(asn string) string {
			if resolvers == nil || resolvers.Cymru == nil {
				return ""
			}
			name, _ := resolvers.Cymru.LookupASNName(context.Background(), parseASNForDecorator(asn))
			return name
		},
	}

	// When TLS is enabled, load or generate cert from blob storage. The SAN
	// hint is derived from the first endpoint; GenerateWebCertWithAddr
	// already fans out to all interface IPs when the host is 0.0.0.0.
	if useTLS {
		if !storage.IsBlobStorage(store) {
			fmt.Fprintf(os.Stderr, "error: looking glass TLS requires blob storage (run ze init first)\n")
			return nil
		}
		certStore := &blobCertStore{store: store}
		certPEM, keyPEM, err := zeweb.LoadOrGenerateCert(certStore, listenAddrs[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: looking glass TLS cert: %v\n", err)
			return nil
		}
		cfg.CertPEM = certPEM
		cfg.KeyPEM = keyPEM
	}

	srv, err := lg.NewLGServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: looking glass disabled: %v\n", err)
		return nil
	}

	// Component startup goroutine (one-time, same pattern as startWebServer).
	serveLG(srv)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if waitErr := srv.WaitReady(readyCtx); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: looking glass server failed to start: %v\n", waitErr)
		_ = srv.Shutdown(context.Background())
		return nil
	}

	scheme := "http"
	if cfg.TLS {
		scheme = "https"
	}
	for _, addr := range srv.Addresses() {
		fmt.Fprintf(os.Stderr, "looking glass listening on %s://%s/\n", scheme, addr)
	}
	return srv
}

// serveLG runs the LG server's ListenAndServe in a background goroutine.
// This is a one-time component startup, not a per-event goroutine.
func serveLG(srv *lg.LGServer) {
	go serveLGBlocking(srv)
}

func serveLGBlocking(srv *lg.LGServer) {
	if serveErr := srv.ListenAndServe(context.Background()); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		slogutil.Logger("lg.server").Error("looking glass server error", "error", serveErr)
	}
}

// serveGNMI runs the gNMI server's Serve in a background goroutine.
// This is a one-time component startup, not a per-event goroutine.
func serveGNMI(ctx context.Context, srv *zegnmi.Server) {
	if serveErr := srv.Serve(ctx); serveErr != nil {
		slogutil.Logger("gnmi.server").Error("gNMI server error", "error", serveErr)
	}
}

// waitForGNMIBind polls until the gNMI server has a bound address or ctx expires.
func waitForGNMIBind(ctx context.Context, srv *zegnmi.Server) bool {
	for {
		if addr := srv.Address(); addr != "" {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(10 * time.Millisecond):
		}
	}
}
