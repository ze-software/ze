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
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/component/ssh"
	"codeberg.org/thomas-mangin/ze/internal/component/web"
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

	fs.Usage = usage

	if err := fs.Parse(args); err != nil {
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
		certPEM, keyPEM, err = web.GenerateWebCertWithAddr(*listen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: generating self-signed certificate: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "using auto-generated self-signed certificate\n")
	}

	// Load user credentials from zefs (created by ze init).
	users, err := loadZefsUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading credentials: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'ze init' to create credentials\n")
		return 1
	}

	// Create renderer for templates and static assets.
	renderer, err := web.NewRenderer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: initializing renderer: %v\n", err)
		return 1
	}

	srv, err := web.NewWebServer(web.WebConfig{
		ListenAddr: *listen,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Wire authentication and routes.
	store := web.NewSessionStore()

	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		if renderErr := renderer.RenderLogin(w, web.LoginData{}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	contentHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parsed, parseErr := web.ParseURL(r)
		if parseErr != nil {
			http.Error(w, parseErr.Error(), http.StatusBadRequest)
			return
		}

		if parsed.Format == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, writeErr := fmt.Fprintf(w, `{"status":"ok","mode":"standalone"}`); writeErr != nil {
				return // client disconnected
			}
			return
		}

		if renderErr := renderer.RenderLayout(w, web.LayoutData{
			Title:      "Ze Web",
			HasSession: true,
		}); renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	})

	loginHandler := web.LoginHandler(store, users, loginRenderer)
	authMiddleware := web.AuthMiddleware(store, users, loginRenderer, contentHandler)
	assetHandler := http.StripPrefix("/assets/", renderer.AssetHandler())

	srv.HandleFunc("POST /login", loginHandler)
	srv.Handle("/assets/", assetHandler)
	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}
		authMiddleware.ServeHTTP(w, r)
	})

	fmt.Fprintf(os.Stderr, "web server starting on https://%s/\n", *listen)

	if err := srv.ListenAndServe(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
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
  --cert <path>    TLS certificate PEM file
  --key <path>     TLS private key PEM file (required with --cert)
  --listen <addr>  Listen address (default: 0.0.0.0:8443)

When --cert is not provided, a self-signed certificate is generated automatically.
Credentials are loaded from zefs (run 'ze init' first).

Examples:
  ze web                                       Start with auto-generated cert
  ze web --listen 0.0.0.0:8443                 Listen on all interfaces
  ze web --cert server.pem --key server-key.pem  Use provided certificate
`)
}
