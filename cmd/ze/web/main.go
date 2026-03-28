// Design: docs/architecture/web-interface.md -- CLI entry point for web interface
//
// Package web provides the ze web subcommand.
package web

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Run executes the ze web command with the given arguments.
// Returns an exit code.
func Run(args []string) int {
	// Check for help first
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return 0
	}

	fs := flag.NewFlagSet("ze web", flag.ContinueOnError)
	cert := fs.String("cert", "", "Path to TLS certificate PEM file")
	key := fs.String("key", "", "Path to TLS private key PEM file")
	listen := fs.String("listen", "0.0.0.0:8443", "Listen address (host:port)")
	insecure := fs.Bool("insecure-web", false, "Disable authentication (testing only)")

	fs.Usage = usage

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// --insecure-web requires 127.0.0.1 to prevent accidental exposure.
	if *insecure && !strings.HasPrefix(*listen, "127.0.0.1:") {
		fmt.Fprintf(os.Stderr, "error: --insecure-web requires --listen 127.0.0.1:<port>\n")
		return 1
	}

	var certPEM, keyPEM []byte

	if *cert != "" {
		// User-supplied certificate: both --cert and --key are required
		if *key == "" {
			fmt.Fprintf(os.Stderr, "error: --key is required when --cert is specified\n")
			return 1
		}

		var err error
		certPEM, err = os.ReadFile(*cert)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading certificate: %v\n", err)
			if os.IsNotExist(err) {
				return 2
			}
			return 1
		}

		keyPEM, err = os.ReadFile(*key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading key: %v\n", err)
			if os.IsNotExist(err) {
				return 2
			}
			return 1
		}
	} else {
		if *key != "" {
			fmt.Fprintf(os.Stderr, "error: --cert is required when --key is specified\n")
			return 1
		}

		// No certificate provided: generate a self-signed one
		var err error
		certPEM, keyPEM, err = zeweb.GenerateWebCertWithAddr(*listen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: generating self-signed certificate: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "using auto-generated self-signed certificate\n")
	}

	// Load user credentials from zefs (created by ze init).
	var users []ssh.UserConfig
	if !*insecure {
		var loadErr error
		users, loadErr = loadZefsUsers()
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "error: loading credentials: %v\n", loadErr)
			fmt.Fprintf(os.Stderr, "hint: run 'ze init' to create credentials, or use --insecure-web for testing\n")
			return 1
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: authentication disabled (--insecure-web)\n")
	}

	// Create renderer for templates and static assets.
	renderer, err := zeweb.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: initializing renderer: %v\n", err)
		return 1
	}

	srv, err := zeweb.NewWebServer(zeweb.WebConfig{
		ListenAddr: *listen,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Load YANG schema for config tree navigation.
	schema, schemaErr := zeconfig.YANGSchema()
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", schemaErr)
		return 1
	}
	tree := zeconfig.NewTree()

	// Use filesystem storage for standalone mode.
	store := storage.NewFilesystem()

	// Find or create a config file for the editor.
	configPath := resolveConfigPath()
	if !store.Exists(configPath) {
		dir := filepath.Dir(configPath)
		if dir != "." && dir != "/" {
			_ = os.MkdirAll(dir, 0o750)
		}
		if writeErr := store.WriteFile(configPath, []byte("# ze config\n"), 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot create config: %v\n", writeErr)
		}
	}
	editorMgr := zeweb.NewEditorManager(store, configPath, schema)

	// CLI completer for Tab/? autocomplete.
	completer := cli.NewCompleter()

	// SSE broker for live config change notifications.
	broker := zeweb.NewEventBroker(0)
	defer broker.Close()

	// Wire authentication.
	sessionStore := zeweb.NewSessionStore()
	loginRenderer := func(w http.ResponseWriter, _ *http.Request) {
		if renderErr := renderer.RenderLogin(w, zeweb.LoginData{}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	var authWrap func(http.Handler) http.Handler
	if *insecure {
		authWrap = zeweb.InsecureMiddleware
	} else {
		authWrap = func(h http.Handler) http.Handler {
			return zeweb.AuthMiddleware(sessionStore, users, loginRenderer, h)
		}
	}

	// Handlers.
	fragmentHandler := zeweb.HandleFragment(renderer, schema, tree, editorMgr, *insecure)
	setHandler := zeweb.HandleConfigSet(editorMgr, schema, renderer)
	deleteHandler := zeweb.HandleConfigDelete(editorMgr)
	commitHandler := zeweb.HandleConfigCommit(editorMgr, renderer, broker)
	discardHandler := zeweb.HandleConfigDiscard(editorMgr)
	cliHandler := zeweb.HandleCLICommand(editorMgr, schema, renderer)
	completeHandler := zeweb.HandleCLIComplete(completer)
	terminalHandler := zeweb.HandleCLITerminal(editorMgr)
	modeHandler := zeweb.HandleCLIModeToggle(editorMgr, schema, renderer)

	// Diff handlers.
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
	diffCloseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html := renderer.RenderFragment("diff_modal", nil)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	})

	// Admin command tree (nil dispatcher in standalone mode -- no BGP engine).
	adminChildren := zeweb.BuildAdminCommandTree()
	adminViewHandler := zeweb.HandleAdminView(renderer, adminChildren)
	adminExecHandler := zeweb.HandleAdminExecute(renderer, nil)

	// Register routes.
	loginHandler := zeweb.LoginHandler(sessionStore, users, loginRenderer)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	srv.Handle("/events", authWrap(broker))
	srv.Handle("/admin/", authWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			adminExecHandler(w, r)
			return
		}
		adminViewHandler(w, r)
	})))
	srv.Handle("POST /cli", authWrap(cliHandler))
	srv.Handle("/cli/complete", authWrap(completeHandler))
	srv.Handle("POST /cli/terminal", authWrap(terminalHandler))
	srv.Handle("POST /cli/mode", authWrap(modeHandler))
	srv.Handle("/fragment/detail", authWrap(fragmentHandler))
	srv.Handle("POST /config/set/", authWrap(setHandler))
	srv.Handle("POST /config/delete/", authWrap(deleteHandler))
	srv.Handle("/config/diff", authWrap(diffHandler))
	srv.Handle("/config/diff-close", authWrap(diffCloseHandler))
	srv.Handle("/config/commit", authWrap(commitHandler))
	srv.Handle("POST /config/discard", authWrap(discardHandler))
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}
		authWrap(fragmentHandler).ServeHTTP(w, r)
	})

	fmt.Fprintf(os.Stderr, "web server starting on https://%s/\n", *listen)

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nshutting down...\n")
		_ = srv.Shutdown(context.Background())
		<-sigCh
		os.Exit(1) // Second signal: force exit.
	}()

	if err := srv.ListenAndServe(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// resolveConfigPath returns a config file path for standalone web editing.
func resolveConfigPath() string {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return "ze.conf"
	}
	return filepath.Join(dir, "ze.conf")
}

// loadZefsUsers reads credentials from the zefs database (created by ze init).
func loadZefsUsers() ([]ssh.UserConfig, error) {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return nil, fmt.Errorf("cannot resolve config directory")
	}

	dbPath := filepath.Join(dir, "database.zefs")

	db, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer db.Close() //nolint:errcheck // read-only access

	username, err := db.ReadFile("meta/ssh/username")
	if err != nil {
		return nil, fmt.Errorf("read username: %w", err)
	}

	hash, err := db.ReadFile("meta/ssh/password")
	if err != nil {
		return nil, fmt.Errorf("read password hash: %w", err)
	}

	name := string(username)
	if name == "" {
		return nil, fmt.Errorf("empty username in zefs")
	}

	return []ssh.UserConfig{{Name: name, Hash: string(hash)}}, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze web [options]

Start the Ze web interface.

Options:
  --cert <path>      TLS certificate PEM file
  --key <path>       TLS private key PEM file (required with --cert)
  --listen <addr>    Listen address (default: 0.0.0.0:8443)
  --insecure-web     Disable authentication (requires --listen 127.0.0.1:*)

When --cert is not provided, a self-signed certificate is generated automatically.
Credentials are loaded from zefs (run 'ze init' first).

Examples:
  ze web                                              Start with auto-generated cert
  ze web --listen 0.0.0.0:8443                        Listen on all interfaces
  ze web --cert server.pem --key server-key.pem       Use provided certificate
  ze web --insecure-web --listen 127.0.0.1:8443       Test without authentication
`)
}
