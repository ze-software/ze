//go:build ze_web

// Design: ai/rules/plugins.md -- compile-out-able web service
// Overview: main_servers.go -- shared hub server helpers
package hub

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
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
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type webService struct {
	*zeweb.WebServer
	broker     *zeweb.EventBroker
	editors    *zeweb.EditorManager
	stopOnce   sync.Once
	stopped    chan struct{}
	stopError  error
	drainLimit time.Duration
}

// webDrainLimit reserves the same share of the process teardown margin as SSH
// model cleanup. It bounds the diagnostic, not the strict borrowed-store join.
const webDrainLimit = rpc.DrainTeardownMarginSeconds * time.Second / 6

var errWebHistoryDrainOverdue = opererr.New("Web cleanup is still waiting for an active request, an owned route resource or a command-history write.",
	"The configuration store remains owned until all accepted work finishes.", webCleanupEventFix)

func newWebService(server *zeweb.WebServer, broker *zeweb.EventBroker, editors *zeweb.EditorManager) *webService {
	return &webService{WebServer: server, broker: broker, editors: editors, stopped: make(chan struct{}), drainLimit: webDrainLimit}
}

func (*webService) Name() string { return "web" }

// Shutdown owns one component-lifecycle cleanup worker. It MUST join handlers
// before draining histories and MUST complete before the caller releases their
// borrowed store. Network failure cannot skip that cleanup. A stuck handler or
// write keeps its owner alive; exceeding the allowance logs a visible error and
// keeps joining. The second stop signal or service-manager timeout can force
// process exit, but a returning caller can never race accepted history against
// store release. Repeated calls wait for the same cleanup.
func (s *webService) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() {
		go func() {
			defer close(s.stopped)
			if s.broker != nil {
				s.broker.Close()
			}
			serverError := s.WebServer.Shutdown(ctx)
			if networkError := zeweb.ShutdownNetworkError(serverError); networkError != nil {
				reportWebNetworkCleanupFailure(networkError)
			}
			var historyError error
			if s.editors != nil {
				historyError = s.editors.Close()
			}
			s.stopError = errors.Join(serverError, historyError)
			if historyError != nil {
				slogutil.Logger("web.server").Error("web command history cleanup failed", "code", diagnostic.CodeDoctorCLIHistoryWrite, "fix", "ze show doctor", "error", historyError)
			}
		}()
	})
	allowance := s.drainLimit
	if deadline, found := ctx.Deadline(); found {
		allowance += max(time.Until(deadline), 0)
	}
	timer := time.NewTimer(allowance)
	defer timer.Stop()
	select {
	case <-s.stopped:
		return s.stopError
	case <-timer.C:
		message := "Web cleanup is still waiting for an active request, an owned route resource or a command-history write; the configuration store remains owned until cleanup finishes. Inspect recent web.server errors to locate the blocked work."
		report.RaiseErrorFixOnce(webCleanupEventSource, diagnostic.CodeDoctorWebCleanupOverdue, webCleanupEventSubject,
			message, webCleanupEventFix, webCleanupEventWindow)
		slogutil.Logger("web.server").Error(message, "code", diagnostic.CodeDoctorWebCleanupOverdue, "fix", webCleanupEventFix, "limit", allowance)
		<-s.stopped
		return errors.Join(errWebHistoryDrainOverdue, s.stopError)
	}
}

func buildWebService(deps *serviceDeps) (Service, error) {
	if !deps.WebEnabled {
		return nil, nil //nolint:nilnil // not-configured is an intentional skip
	}
	// A local, not a write back into deps: buildServices hands one struct to
	// every factory, so a rewritten field would reach the next one.
	webAddrs := resolveWebListeners(true, deps.WebAddrs)
	for _, svc := range deps.WebPortalServices {
		zeweb.RegisterPortalService(zeweb.PortalService{Key: svc.Key, Title: svc.Title, Path: svc.Path, Icon: svc.Icon})
	}
	webSrv, broker, editors := startWebServer(
		deps.Store,
		deps.ConfigPath,
		webAddrs,
		deps.InsecureWeb,
		deps.WebCertificate,
		deps.Dispatch,
		deps.Resolvers,
		deps.Authorizer,
		deps.Recorder,
		deps.ConfigOwner,
		deps.PowerUsers,
		deps.LocalUsersLive,
		deps.WebCommands,
		deps.WebArgumentTree,
		deps.OperationalCommand,
		deps.Streams,
	)
	if webSrv == nil {
		return nil, nil //nolint:nilnil // startWebServer preserves prior best-effort skip behavior
	}
	if ring := deps.EventRing; ring != nil {
		ring.Append("web", "server.started")
		wireEventRingToBroker(ring, broker)
	}
	return newWebService(webSrv, broker, editors), nil
}

func runWebOnly(store storage.Storage, listenAddr string, insecureWeb bool) (exit int) {
	resolvers, unwatchResolvers := newResolvers(&system.SystemConfig{DNSTimeout: 5, DNSCacheSize: 10000, DNSCacheTTL: 86400})
	defer resolvers.Close()
	defer unwatchResolvers()
	// Web-only mode runs no daemon and learns no lease: the host resolv.conf
	// is its only source of resolvers. No path: it writes no file.
	if err := system.ProcessResolvers().Configure("", nil, system.DefaultRouterDNSSource()); err != nil {
		slogutil.Logger("web").Warn("resolvers not configured", "err", err)
	}
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
	commands, commandErr := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if commandErr != nil {
		refusal := opererr.Wrap(commandErr, "The web interface could not prepare its commands.",
			"No web server was started, and no configuration was applied.", "ze show doctor")
		fmt.Fprintf(os.Stderr, "error: %v\n", refusal)
		return 1
	}
	sigCh := make(chan os.Signal, 2) //nolint:mnd // buffer 2: graceful + force
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	var service *webService
	var forceDone chan struct{}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		cleanupError := shutdownWebOnly(shutdownCtx, service, commands)
		// Accepted response callbacks finish in the web join, so neither a
		// command nor signal delivery can send after this channel closes.
		releaseForceSignals(sigCh)
		if forceDone != nil {
			<-forceDone
		}
		if cleanupError != nil {
			refusal := opererr.Wrap(cleanupError, "The web interface could not finish cleanup.",
				"Web requests or command-owned resources did not stop cleanly; store recovery is not confirmed.", "ze show doctor")
			fmt.Fprintf(os.Stderr, "error: %v\n", refusal)
			exit = 1
		}
		if exit == 0 {
			exit = webOnlyExitStatus(cleanupError)
		}
	}()
	bindWebOnlyRecovery(store, commands, sigCh)
	ring := commands.EventRing()
	ring.Append("web", "server.started")
	dispatch := webOnlyDispatcher(commands)
	auditLog, auditErr := openAuditLog("")
	if auditErr != nil {
		fmt.Fprintf(os.Stderr, "error: audit log: %v\n", auditErr)
		return 1
	}
	showCmd.RegisterAuditProvider(auditLog.Query)
	commands.Dispatcher().SetAuditRecorder(auditLog)
	// Web-only mode runs before any config is loaded, so there are no
	// config-file users (only the zefs power user authenticates here), no
	// plugin engine to source runtime completions from, and no pki
	// block to reference: the self-signed certificate path is the only one.
	// noConfigUsers states that emptiness rather than leaving it implied.
	//
	// This is the one process that reads zefs for the web server, because there
	// is no AAA chain here to share the read with. An unreadable database leaves
	// the power user out of both the serve-or-not test and the live view, which
	// is the same answer both would reach separately.
	powerUsers, powerErr := usersFromStore(store)
	if powerErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web power-user auth unavailable: %v\n", powerErr)
	}
	localUsersLive := liveLocalUsers(powerUsers, noConfigUsers, slogutil.Logger("hub.web.auth"))
	webCommands := func() []command.CommandEntry { return commands.Dispatcher().Registry().VisibleCommandEntries() }
	webSrv, broker, editors := startWebServer(store, "", listenAddrs, insecureWeb, "", dispatch, resolvers, nil, auditLog, nil, powerUsers, localUsersLive, webCommands, commands.WithArgCompletion, serverOperationalCommand(commands))
	if webSrv == nil {
		return 1
	}
	wireEventRingToBroker(ring, broker)
	service = newWebService(webSrv, broker, editors)

	writeReadyFile()

	fmt.Println("Ze web running. Press Ctrl+C to stop.")
	<-sigCh
	fmt.Println("\nShutting down (Ctrl+C again to force)...")

	// Second signal forces immediate exit (lifecycle goroutine, not hot path).
	forceDone = make(chan struct{})
	go func() {
		defer close(forceDone)
		forceExitOnSignal(sigCh)
	}()

	return 0
}

// webOnlyDispatcher uses the native command registry without starting a daemon.
// An absent command keeps the standalone interface's operator guidance.
func webOnlyDispatcher(server *pluginserver.Server) zeweb.CommandDispatcher {
	dispatch := serverDispatcher(server, audit.Web)
	return func(ctx context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
		response, err := dispatch(ctx, caller, input)
		if errors.Is(err, pluginserver.ErrUnknownCommand) {
			return nil, errWebOnlyUnavailable
		}
		return response, err
	}
}

// errWebOnlyUnavailable is returned by webOnlyDispatcher for any operational
// command that needs the daemon. The message is written for direct display in
// the web UI (tools render dispatch errors inline; log pages map it to an
// honest empty state) rather than exposing the raw command string (F4/AC-10).
var errWebOnlyUnavailable = opererr.New("This command needs a running daemon with a loaded configuration.",
	"The standalone web interface cannot answer it from configuration editing alone.", "ze start")

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

// startWebServer creates and starts the web server with zefs credentials.
// Returns the server and SSE event broker on success, nil on failure (logged, non-fatal).
// Caller MUST call broker.Close() during shutdown to release SSE clients.
// Every entry in listenAddrs becomes a bound listener on the same
// *http.Server; Shutdown closes all of them.
// Requires the store: TLS keys and config must not leak to loose files.
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
func startWebServer(store storage.Storage, configPath string, listenAddrs []string, insecureWeb bool, certName string, dispatch zeweb.CommandDispatcher, resolvers *resolve.Resolvers, authorizer aaa.Authorizer, recorder audit.Recorder, configOwner *configMutationOwner, powerUsers []authz.UserConfig, localUsersLive func() ([]authz.UserConfig, error), commandEntries func() []command.CommandEntry, argumentTree func(*command.Node, command.ArgumentQuery) *command.Node, registered func(string) bool, streams ...plugin.CommandStreamHandlers) (*zeweb.WebServer, *zeweb.EventBroker, *zeweb.EditorManager) {
	if insecureWeb {
		authorizer = nil
	}

	if store == nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: persistent store unavailable (run ze init first)\n")
		return nil, nil, nil
	}

	listenAddrs = resolveWebListeners(true, listenAddrs)

	powerUserNames := make([]string, 0, len(powerUsers))
	for _, u := range powerUsers {
		powerUserNames = append(powerUserNames, u.Name)
	}
	if insecureWeb {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
	} else {
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
			return nil, nil, nil
		}
		serving, usersErr := localUsersLive()
		if usersErr != nil {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: cannot read the running configuration users: %v\n", usersErr)
			return nil, nil, nil
		}
		if len(serving) == 0 {
			fmt.Fprintf(os.Stderr, "warning: web server disabled: no authenticatable users\n")
			return nil, nil, nil
		}
	}

	// Persist the self-signed cert in zefs so browsers don't have to re-accept
	// on every restart. A named certificate never reaches this store.
	certStore := &blobCertStore{store: store}
	certPEM, keyPEM, err := listenerTLSMaterial(certName, certStore, listenAddrs[0])
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
		return nil, nil, nil
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: YANG schema: %v\n", schemaErr)
		return nil, nil, nil
	}

	// The operational command tree comes from the same registered modules as
	// the schema, and DefaultLoader is strict: a module that fails to load
	// disables the server, as a schema failure does, rather than serving a UI
	// whose command tree silently lacks it.
	resolved, loaderErr := yangloader.DefaultLoader()
	if loaderErr != nil {
		var tb textbuf.Buffer
		tb.Str("warning: web server disabled: operational command tree: ").Err(loaderErr).Byte('\n').StdErr() //nolint:errcheck // a warning to stderr has nowhere further to report
		return nil, nil, nil
	}
	commandTree := yangloader.BuildCommandTree(resolved)
	// Strict ze:related validation against the full operational command
	// tree. Surfaces typos and renamed-command drift at hub startup so
	// operators see the diagnostic before any workbench click. Logged as
	// a warning (not fatal) so a single drifted descriptor never prevents
	// the hub from serving the rest of the UI.
	if validateErr := zeconfig.ValidateSchemaAgainstCommandTree(schema, commandTree); validateErr != nil {
		var tb textbuf.Buffer
		tb.Str("warning: ze:related validation: ").Err(validateErr).Byte('\n').StdErr() //nolint:errcheck // a warning to stderr has nowhere further to report
	}

	// Ensure a config file exists for the editor.
	if configPath == "" {
		name, err := internalresolve.DefaultConfig(store)
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: web server disabled:", err)
			return nil, nil, nil
		}
		configPath = name
	}
	if !store.Exists(configPath) {
		if writeErr := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create config: %v\n", writeErr)
		}
	}

	tree := committedBaselineTree(store, configPath)

	// Create editor manager for config editing via web.
	var editorMgr *zeweb.EditorManager = zeweb.NewEditorManager(store, configPath, schema, newEditorFactory(zeconfigcmd.ValidateContent), newEditSessionFactory()) //nolint:modernize // inferdecl: Keep the concrete editor-owner type so the error inventory can prove Diff's operator-error result.
	var broker *zeweb.EventBroker
	managerOwned := true
	defer func() {
		if managerOwned {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			// The constituent producers report network, history and overdue
			// failures; startup failure still strictly joins borrowed-store work.
			_ = newWebService(srv, broker, editorMgr).Shutdown(ctx) //nolint:errcheck // Cleanup failures are reported by their producers.
		}
	}()
	var commitHook func() error
	if configOwner != nil {
		commitHook = func() error { return configOwner.Reload(context.Background()) }
	}
	if commitHook != nil {
		// Install before serving so early commits cannot bypass daemon reload.
		editorMgr.SetCommitHook(commitHook)
		// The web commits through the daemon's confirmed-commit window, the
		// one the SSH session editors use (cli.WindowCommit).
		editorMgr.SetConfirmWindow(daemonConfirmWindow.Load)
		editorMgr.SetConfigSource(func() ([]byte, error) {
			return storage.ReadConfigSource(store, configPath)
		}, func(expected, content []byte) error {
			return configOwner.Commit(context.Background(), expected, content)
		})
	}

	// Plugin commands are overlaid live per completion request (plugins
	// register after the web service is built, and may come and go), so the
	// YANG tree stays immutable and completion always reflects the current
	// registry. YANG-only when there is no plugin source (web-only mode).
	completionSource := webCompletionSource(commandTree, commandEntries, argumentTree, dispatch)
	// Create CLI completer for Tab/? autocomplete.
	completer, err := cli.NewCompleter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web server disabled: completer: %v\n", err)
		return nil, nil, nil
	}

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
	broker = zeweb.NewEventBroker(0)
	if commitHook != nil {
		// Web users see the daemon window's countdown and how it ended, and a
		// forced commit's discard notice, as the SSH draft poll shows them.
		// Run ends when the broker closes (webService.Shutdown).
		go zeweb.NewWindowNotices(editorMgr, broker).Run()
	}

	// The graphical YANG editor is primary. The unfinished Workbench requires
	// ze.web.ui-mode=workbench or an explicit current browser switch cookie.
	uiMode := zeweb.GetUIMode()
	adminTrees := adminCommandTrees(commandTree, commandEntries)
	finderHandler := zeweb.HandleFragment(renderer, schema, tree, editorMgr, insecureWeb, authorizer, adminTrees)
	workbenchHandler := zeweb.HandleWorkbench(renderer, schema, tree, editorMgr, insecureWeb,
		zeweb.WithDispatch(dispatch), zeweb.WithBroker(broker), zeweb.WithPowerUsers(powerUserNames),
		zeweb.WithAuthorizer(authorizer))
	fmt.Fprintf(os.Stderr, "web UI default: %s\n", uiMode)
	showHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch zeweb.ReadUIModeFromRequest(r, uiMode) {
		case zeweb.UIModeWorkbench:
			workbenchHandler(w, r)
		case zeweb.UIModeFinder:
			finderHandler(w, r)
		default:
			panic("BUG: unknown web UI mode")
		}
	})
	// Fragment handler still serves /fragment/detail HTMX requests regardless
	// of mode; both UIs share the same OOB swap path.
	fragmentHandler := finderHandler

	// Config set, add, and delete handlers for editing leaf values.
	setHandler := zeweb.HandleConfigSetWithAuthorizer(editorMgr, schema, renderer, authorizer)
	actionHandler := zeweb.HandleGraphicalAction(editorMgr, schema, renderer, authorizer, recorder, broker, adminTrees)
	importHandler := zeweb.HandleGraphicalImport(editorMgr, schema, renderer, authorizer, adminTrees)
	inspectionFormHandler := zeweb.HandleSuppliedInspectionForm(renderer, adminTrees)
	inspectionHandler := zeweb.HandleSuppliedInspection(renderer, editorMgr, dispatch, adminTrees, authorizer)
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
		diff, err := editorMgr.Diff(username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
	cliHandler := zeweb.HandleCLICommandWithAuthorizerAndAudit(editorMgr, schema, renderer, authorizer, recorder, dispatch)
	terminalHandler := zeweb.HandleCLITerminalWithDispatchAuthorizerAndAudit(editorMgr, schema, tree, dispatch, authorizer, recorder, registered)
	var streamHandlers plugin.CommandStreamHandlers
	if len(streams) > 0 {
		streamHandlers = streams[0]
	}
	views := zeweb.NewCLIViewTransport(streamHandlers.Admit, webViewBinding(dispatch))
	streamHandler := views.Handler(editorMgr, terminalHandler, streamHandlers.Dispatch, registered)
	modeHandler := zeweb.HandleCLIModeToggle(editorMgr, schema, renderer)
	completeHandler := zeweb.HandleCLICompleteWithSource(completer, completionSource, registered, editorMgr, schema)

	// The live AAA chain decides ordinary logins once installed. Only an
	// unavailable chain permits ordinary local fallback; an explicit remote
	// rejection can be overridden solely by the local reserved recovery
	// profile that usersFromStore assigns to the ZeFS super-admin.
	// localUsersLive reads the accepted generation, so removed or replaced
	// credentials cannot regain access through this fallback after reload.
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

	// The admin console's tree: the YANG operational command tree with the live
	// plugin registry overlaid per request (adminCommandTrees). Each command's
	// declared summary and explanation reach its form. A loader failure
	// disabled the server above rather than serving broken admin links.
	adminViewHandler := zeweb.HandleAdminView(renderer, adminTrees)
	adminExecHandler := zeweb.HandleAdminExecute(renderer, dispatch, adminTrees)

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
	srv.Handle("GET /cli", authWrap(zeweb.HandleCLIPageHTTP(renderer, insecureWeb, editorMgr)))
	srv.Handle("POST /cli", mutationWrap(cliHandler))
	srv.Handle("/cli/complete", authWrap(completeHandler))
	srv.Handle("POST /cli/terminal", mutationWrap(terminalHandler))
	srv.Handle("POST /cli/stream", mutationWrap(streamHandler))
	srv.Handle("POST /cli/view/action", mutationWrap(http.HandlerFunc(views.Action)))
	srv.Handle("POST /cli/mode", mutationWrap(modeHandler))
	srv.Handle("/fragment/detail", authWrap(fragmentHandler))
	srv.Handle("POST /config/set/", mutationWrap(setHandler))
	srv.Handle("POST /config/action", mutationWrap(actionHandler))
	srv.Handle("POST /config/import", mutationWrap(importHandler))
	srv.Handle("GET /config/inspect/form", authWrap(inspectionFormHandler))
	srv.Handle("POST /config/inspect", mutationWrap(inspectionHandler))
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
	// its portal-menu entry when wired. A route that authorizes per request
	// (the gokrazy proxy judges each page by the Ze command it needs) reads
	// the authorizer from routeDeps.
	routeWraps := map[zeweb.WrapKind]func(http.Handler) http.Handler{
		zeweb.WrapAuth:     authWrap,
		zeweb.WrapMutation: mutationWrap,
	}
	routeDeps := zeweb.RouteDeps{Renderer: renderer, Dispatch: dispatch, Authorizer: authorizer,
		EditorManager: editorMgr, Recorder: recorder}
	for _, route := range zeweb.RegisteredWebRoutes() {
		if route.Enabled != nil && !route.Enabled() {
			continue
		}
		handler := route.Build(routeDeps)
		srv.OwnHandler(handler)
		srv.Handle(route.Pattern, routeWraps[route.Wrap](handler))
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
		return nil, nil, nil
	}

	fmt.Fprintf(os.Stderr, "web server listening on https://%s/\n", srv.Address())
	managerOwned = false
	return srv, broker, editorMgr
}

// forceExitOnSignal waits for a second OS signal and exits immediately.
// The standalone web service starts it once during shutdown, for an impatient
// Ctrl+C. The caller MUST stop delivery, close sigCh and join this worker before
// returning its borrowed resources. Command-requested stops never force exit.
func forceExitOnSignal(sigCh <-chan os.Signal) {
	// The caller's channel close ends this one process-lifecycle worker.
	for event := range sigCh {
		if _, systemSignal := event.(syscall.Signal); !systemSignal {
			continue
		}
		stopForceExit()
		return
	}
}

// adminCommandTrees is the web admin console's command tree producer. It
// answers, for each request, base (the YANG operational command tree, which
// holds every builtin, local and local-data command under its YANG path) with
// the plugin commands entries lists overlaid on a copy
// (command.OverlayCommandPaths). entries is the registry's
// VisibleCommandEntries, which asks the surface predicate
// (command.ListedOnDaemonSurfaces), so a plugin command with no YANG node is
// offered, a hidden one is not, and a plugin that registered or exited since
// startup is reflected on the next request. base is shared and never written.
// A nil entries (web-only mode, no plugin registry) answers base alone. Either
// way a streaming command declared streaming-only is removed first
// (streamingPathsOff): the console runs a command once, and such a command
// has no one-shot answer.
func adminCommandTrees(base *command.Node, entries func() []command.CommandEntry) zeweb.AdminCommandTree {
	base = command.WithoutCommandPaths(base, streamingPathsOff(command.SurfaceWebAdmin))
	if entries == nil {
		return zeweb.StaticCommandTree(base)
	}
	return func() *command.Node { return command.OverlayCommandPaths(base, entries()) }
}

// committedBaselineTree parses the committed config into the live baseline
// tree the web compare targets read. Saved drafts are a separate source, not
// the startup-loaded config. The parse publishes nothing
// (zeconfig.ParseTreeForValidation): this is the daemon reading its own
// config again, and the startup loader already applied the file's
// `environment { }` block, so a second application would only override what
// the process set since. An unreadable or unparsable file answers an empty
// tree, which the editor then shows.
func committedBaselineTree(store storage.Storage, configPath string) *zeconfig.Tree {
	configData, readErr := store.ReadFile(configPath)
	if readErr != nil {
		return zeconfig.NewTree()
	}
	parsed, parseErr := zeconfig.ParseTreeForValidation(string(configData))
	if parseErr != nil {
		return zeconfig.NewTree()
	}
	return parsed
}
