//go:build ze_web

// Design: ai/rules/plugins.md -- compile-out-able web service
// Overview: main_servers.go -- shared hub server helpers
package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	showCmd "github.com/ze-software/ze/internal/component/cmd/show"
	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type webService struct {
	*zeweb.WebServer
	broker *zeweb.EventBroker
}

func (webService) Name() string { return "web" }

func (s webService) Shutdown(ctx context.Context) error {
	if s.broker != nil {
		s.broker.Close()
	}
	return s.WebServer.Shutdown(ctx)
}

func buildWebService(deps serviceDeps) (Service, error) {
	if !deps.WebEnabled {
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}
	deps.WebAddrs = resolveWebListeners(true, deps.WebAddrs)
	for _, svc := range deps.WebPortalServices {
		zeweb.RegisterPortalService(zeweb.PortalService{Key: svc.Key, Title: svc.Title, Path: svc.Path, Icon: svc.Icon})
	}
	webSrv, broker := startWebServer(
		deps.Store,
		deps.ConfigPath,
		deps.WebAddrs,
		deps.InsecureWeb,
		deps.WebCertificate,
		deps.Dispatch,
		deps.Resolvers,
		deps.Authorizer,
		deps.Recorder,
		deps.CommitHook,
		deps.PowerUsers,
		deps.LocalUsersLive,
		deps.WebCommands,
	)
	if webSrv == nil {
		return nil, nil //nolint:nilnil // startWebServer preserves prior best-effort skip behavior
	}
	if ring := deps.EventRing; ring != nil {
		ring.Append("web", "server.started")
		wireEventRingToBroker(ring, broker)
	}
	return webService{WebServer: webSrv, broker: broker}, nil
}

func runWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) int {
	resolvers := newResolvers(&system.SystemConfig{DNSTimeout: 5, DNSCacheSize: 10000, DNSCacheTTL: 86400})
	defer resolvers.Close()
	if resolvers.DNS != nil {
		command.SetPTRResolver(resolvers.DNS)
	}
	if resolvers.Cymru != nil {
		command.SetOriginResolver(cymruOriginAdapter{resolvers.Cymru})
	}

	var listenAddrs []string
	if listenAddr != "" {
		listenAddrs = []string{listenAddr}
	}
	ring := pluginserver.NewEventRing(128)
	ring.Append("web", "server.started")
	dispatch := webOnlyDispatcher(ring)
	auditLog, auditErr := openAuditLog("")
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "error: audit log: %v\n", auditErr)
		return 1
	}
	showCmd.RegisterAuditProvider(auditLog.Query)
	// Web-only mode runs before any config is loaded, so there are no
	// config-file users (only the zefs power user authenticates here), no
	// plugin engine to source completion commands from (nil source), and no pki
	// block to reference: the self-signed certificate path is the only one.
	// noConfigUsers states that emptiness rather than leaving it implied.
	//
	// This is the one process that reads zefs for the web server, because there
	// is no AAA chain here to share the read with. An unreadable database leaves
	// the power user out of both the serve-or-not test and the live view, which
	// is the same answer both would reach separately.
	powerUsers, powerErr := loadZefsUsers()
	if powerErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web power-user auth unavailable: %v\n", powerErr)
	}
	localUsersLive := liveLocalUsers(powerUsers, noConfigUsers, slogutil.Logger("hub.web.auth"))
	webSrv, broker := startWebServer(store, "", listenAddrs, insecureWeb, "", dispatch, resolvers, nil, auditLog, nil, powerUsers, localUsersLive, nil)
	if webSrv == nil {
		return 1
	}
	wireEventRingToBroker(ring, broker)

	sigCh := make(chan os.Signal, 2) //nolint:mnd // buffer 2: graceful + force
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Ze web running. Press Ctrl+C to stop.")
	<-sigCh
	fmt.Println("\nShutting down (Ctrl+C again to force)...")

	// Second signal forces immediate exit (lifecycle goroutine, not hot path).
	go forceExitOnSignal(sigCh)

	broker.Close()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = webSrv.Shutdown(shutdownCtx)

	return 0
}

// webOnlyDispatcher creates a minimal CommandDispatcher backed by a local event
// ring. Used by RunWebOnly where no plugin server exists.
func webOnlyDispatcher(ring *pluginserver.EventRing) zeweb.CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
		switch {
		case strings.HasPrefix(command, "show event namespaces"):
			counts := ring.NamespaceCounts()
			rows := make([]map[string]any, 0, len(counts))
			for ns, count := range counts {
				rows = append(rows, map[string]any{"namespace": ns, "count": count})
			}
			b, err := json.Marshal(map[string]any{"namespaces": rows})
			if err != nil {
				return nil, err
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(b)), nil

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
				return nil, err
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(b)), nil

		default:
			return nil, errWebOnlyUnavailable
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
//
// The decoder is reached through the leaf registry seam, which the gated BGP
// CLI package fills from its init(). With ze_bgp compiled out the seam is nil
// and the command falls through to the dispatcher, which reports it as unknown
// -- the honest answer for a binary that has no BGP.
func withBGPDecode(inner zeweb.CommandDispatcher) zeweb.CommandDispatcher {
	const prefix = "show bgp decode "
	return func(ctx context.Context, caller plugin.CallerIdentity, command string) (*plugin.Response, error) {
		if decode := pluginreg.GetPacketDecoder(); decode != nil && strings.HasPrefix(command, prefix) {
			hex := strings.TrimSpace(command[len(prefix):])
			// The decoder is asked for JSON (outputJSON=true) so the response
			// carries structured data. A payload that is already rendered text
			// leaves "| json", "| yaml" and "| table" nothing to work with, so
			// the rendering is the pipe layer's job here, never the decoder's
			// (ai/rules/cli.md).
			decoded, err := decode(hex, "", "", true)
			if err != nil {
				return nil, err
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
				return nil, fmt.Errorf("decode bgp packet: %w", err)
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.Map(payload)), nil
		}
		if inner != nil {
			return inner(ctx, caller, command)
		}
		return nil, errWebOnlyUnavailable
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

// webTLSMaterial returns the PEM material the web listener serves.
//
// certName is the operator's environment.web.certificate leaf. When it is set,
// the material comes from the PKI store and carries the full chain; when it is
// empty, the established self-signed pair is loaded from (or generated into)
// the blob store, exactly as before this leaf existed.
//
// It FAILS CLOSED: a configured name that does not resolve returns an error and
// no material. It never falls back to the self-signed path, because a listener
// that quietly serves a self-signed certificate while the config names a real
// one is indistinguishable from a working deployment until a client rejects it
// (R-5 of plan/spec-pki-full-chain.md).
func webTLSMaterial(certName string, certStore selfcert.CertStore, listenAddr string) (certPEM, keyPEM []byte, err error) {
	if certName != "" {
		return zepki.ServerTLSMaterial(certName)
	}
	// Persist the self-signed cert in zefs so browsers don't have to re-accept
	// on every restart. The SAN hint is derived from the first endpoint;
	// GenerateWebCertWithAddr already fans out to all interface IPs when the
	// host is 0.0.0.0.
	return selfcert.LoadOrGenerateCert(certStore, listenAddr)
}

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server and SSE event broker on success, nil on failure (logged, non-fatal).
// Caller MUST call broker.Close() during shutdown to release SSE clients.
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
// Requires blob storage -- TLS keys and config must not leak to the filesystem.
// localUsersLive reads the accepted local identity generation shared with AAA,
// SSH, REST, and gRPC. The ConfigProvider may hold a rejectable reload candidate,
// so the web session path must not read it directly.
//
// It is consulted per login attempt and per request carrying a session cookie,
// so a successfully published reload that removes a user takes effect at once on
// both paths, and once here to decide whether the server serves at all.
//
// powerUsers is the caller's zefs snapshot, used to NAME those accounts for the
// UI. It never decides who may log in; localUsersLive already carries them.
func startWebServer(store storage.Storage, configPath string, listenAddrs []string, insecureWeb bool, certName string, dispatch zeweb.CommandDispatcher, resolvers *resolve.Resolvers, authorizer aaa.Authorizer, recorder audit.Recorder, commitHook func() error, powerUsers []authz.UserConfig, localUsersLive func() ([]authz.UserConfig, error), commandEntries func() []command.CommandEntry) (*zeweb.WebServer, *zeweb.EventBroker) {
	dispatch = withBGPDecode(dispatch)
	if insecureWeb {
		authorizer = nil
	}

	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: requires blob storage (run ze init first)\n")
		return nil, nil
	}

	listenAddrs = resolveWebListeners(true, listenAddrs)

	powerUserNames := make([]string, 0, len(powerUsers))
	for _, u := range powerUsers {
		powerUserNames = append(powerUserNames, u.Name)
	}
	if !insecureWeb {
		// Serve only when somebody can log in. Both the zefs power user and
		// config-file users may, and an absent power user is not fatal while
		// config users exist. The question is asked of the SAME source the
		// login path answers from, so the server cannot decide to serve on a
		// user list it will then refuse to admit anyone from.
		//
		// A source that is missing or unreadable disables the server (fail
		// closed -- never serve an unauthenticated admin UI).
		if localUsersLive == nil {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: no user source wired\n")
			return nil, nil
		}
		serving, usersErr := localUsersLive()
		if usersErr != nil {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: cannot read the running configuration users: %v\n", usersErr)
			return nil, nil
		}
		if len(serving) == 0 {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: no authenticatable users\n")
			return nil, nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
	}

	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := webTLSMaterial(certName, certStore, listenAddrs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: TLS cert: %v\n", err)
		return nil, nil
	}

	renderer, err := zeweb.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: renderer: %v\n", err)
		return nil, nil
	}

	// Register display-time decorators (e.g., ASN -> org name via Team Cymru DNS).
	decorators := zeweb.NewDecoratorRegistry()
	if resolvers != nil && resolvers.Cymru != nil {
		decorators.Register(zeweb.NewASNNameDecoratorFromCymru(resolvers.Cymru))
	}
	if resolvers != nil && resolvers.DNS != nil {
		decorators.Register(zeweb.NewReverseDNSDecoratorFromResolver(resolvers.DNS))
	}
	// community-name maps well-known community values to their RFC names via the
	// in-process registry; no external resolver needed.
	decorators.Register(zeweb.NewCommunityNameDecorator())
	renderer.SetDecorators(decorators)

	srv, err := zeweb.NewWebServer(zeweb.WebConfig{
		ListenAddrs: listenAddrs,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: %v\n", err)
		return nil, nil
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: YANG schema: %v\n", schemaErr)
		return nil, nil
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
		// Plugin commands are overlaid live per completion request (plugins
		// register after the web service is built, and may come and go), so the
		// YANG tree stays immutable and completion always reflects the current
		// registry. YANG-only when there is no plugin source (web-only mode).
		if commandEntries != nil {
			commandCompleter = newPluginAwareCommandCompleter(commandTree, commandEntries)
		} else {
			commandCompleter = cli.NewCommandCompleter(commandTree)
		}
	}
	// Create CLI completer for Tab/? autocomplete.
	completer := cli.NewCompleter()

	// The caller's one live view of the local credentials, shared by the session
	// store, by the web fallback authenticator below, and by the AAA chain the
	// caller built it for. Three readers of one producer: a session, a password
	// and the chain must never disagree about who exists.
	sessionStore := zeweb.NewSessionStore(localUsersLive)
	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		data := zeweb.LoginData{ReturnTo: r.URL.RequestURI(), Locale: zeweb.LocaleFromRequest(r)}
		if renderErr := renderer.RenderLogin(w, data); renderErr != nil {
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
		zeweb.WithDispatch(dispatch), zeweb.WithBroker(broker), zeweb.WithPowerUsers(powerUserNames),
		zeweb.WithAuthorizer(authorizer))
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

	// Config download/upload: full-config export/import via browser (AC-3/AC-4).
	// Both go out behind the authz edit permission, download through editWrap
	// and upload through editMutationWrap. The download is the raw config, so it
	// carries every secret a display path masks (internal/component/web/secret.go).
	// Upload validates through the same validator as commit and reloads on
	// success. Both are audit-logged.
	downloadHandler := zeweb.HandleConfigDownload(editorMgr, recorder)
	uploadHandler := zeweb.HandleConfigUpload(editorMgr, zeconfigcmd.ValidateContent, configPath, authorizer, recorder)

	// Diff handler: returns the diff modal HTML (open, with content).
	diffHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := zeweb.GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		diff, _ := editorMgr.Diff(username)
		count := editorMgr.ChangeCount(username)
		html, renderErr := renderer.RenderDiffModalOpen(diff, count)
		if renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// Diff close: returns the closed modal HTML.
	diffCloseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html, renderErr := renderer.RenderDiffModal()
		if renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
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

	// Auth wrapper for protecting individual routes. The live AAA bundle chain
	// (RADIUS/TACACS + local) is preferred once infra setup installs it; before
	// that (web starts before config load in the BGP path) and for users absent
	// from the chain, it falls back to the zefs power user plus the config-file
	// users the RUNNING config declares. This lets RADIUS/TACACS admins
	// authenticate on web without regressing local login (AC-2, A-3), while a
	// user the operator deletes stops authenticating at the next reload rather
	// than at the next restart -- with a password here, and with an already
	// issued session cookie in sessionStore above (AC-10).
	webAuth := liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{UsersFunc: localUsersLive}}
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
	// editWrap gates configuration-editing and admin pages behind the authz
	// edit permission: read-only sessions get 403 (AC-1). RequireEditAuthz runs
	// INSIDE authWrap so the username/profiles context is already set. Fail-open
	// when no authz assignments exist (authorizer allows) preserves single-admin
	// deployments (R-1); insecure/web-only mode has a nil authorizer and is never
	// gated.
	editWrap := func(h http.Handler) http.Handler {
		return authWrap(zeweb.RequireEditAuthz(authorizer, h))
	}
	editMutationWrap := func(h http.Handler) http.Handler {
		return authWrap(zeweb.RequireEditAuthz(authorizer, zeweb.RequireSameOrigin(h)))
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
	// Admin console (operational command execution) is an edit/ops surface:
	// read-only sessions are denied at the route (AC-1), not just per-command.
	srv.Handle("/admin/", editMutationWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv.Handle("GET /config/add-form/", editWrap(addFormHandler))
	srv.Handle("POST /config/rename/", mutationWrap(renameHandler))
	srv.Handle("GET /config/changes", authWrap(zeweb.HandleConfigChanges(editorMgr, renderer, authorizer)))
	srv.Handle("POST /config/delete/", mutationWrap(deleteHandler))
	srv.Handle("/config/diff", authWrap(diffHandler))
	srv.Handle("/config/diff-close", authWrap(diffCloseHandler))
	srv.Handle("/config/commit", mutationWrap(commitHandler))
	srv.Handle("/config/commit/", mutationWrap(commitHandler))
	// Config export and import are BOTH edit-gated: the raw download streams the
	// committed config verbatim, including the real bcrypt password hash, so a
	// read-only session must not fetch it (an unmasked hash is a credential over
	// the local CLI path). editWrap denies read-only sessions (403); upload adds
	// same-origin on top (spec-fixit-bcrypt-hash-credential AC-4).
	srv.Handle("GET /config/download", editWrap(downloadHandler))
	srv.Handle("POST /config/upload", editMutationWrap(uploadHandler))
	srv.Handle("POST /config/discard", mutationWrap(discardHandler))
	srv.Handle("POST /config/discard/", mutationWrap(discardHandler))
	// Workbench related-tool execution. Browser submits only tool id +
	// context path; the handler resolves the descriptor server-side and
	// dispatches via the standard CommandDispatcher (same authz pipeline
	// as /cli and /admin).
	srv.Handle("POST /tools/related/run", mutationWrap(zeweb.HandleRelatedToolRun(renderer, schema, tree, editorMgr, dispatch)))
	// In-tree feature routes (L2TP, IS-IS, OSPF, gokrazy portal) self-register
	// via zeweb.WebRoute in the web package; the hub wraps each by kind and
	// serves it. Adding or removing a feature's routes needs no edit here
	// (AC-5, registration over hardcoding). The wrap helpers stay in the hub
	// because they close over the session store, authorizer, and audit
	// recorder (R-2); a route that returns nil from Enabled (e.g. gokrazy when
	// its service is off) is skipped, and a route carrying a Portal registers
	// its portal-menu entry when wired.
	routeWraps := map[zeweb.WrapKind]func(http.Handler) http.Handler{
		zeweb.WrapAuth:     authWrap,
		zeweb.WrapMutation: mutationWrap,
	}
	routeDeps := zeweb.RouteDeps{Renderer: renderer, Dispatch: dispatch}
	for _, route := range zeweb.RegisteredWebRoutes() {
		if route.Enabled != nil && !route.Enabled() {
			continue
		}
		srv.Handle(route.Pattern, routeWraps[route.Wrap](route.Build(routeDeps)))
		if route.Portal != nil {
			zeweb.RegisterPortalService(*route.Portal)
		}
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
		return nil, nil
	}

	fmt.Fprintf(os.Stderr, "web server listening on https://%s/\n", srv.Address())
	return srv, broker
}
