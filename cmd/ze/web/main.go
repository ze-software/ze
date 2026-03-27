// Design: docs/architecture/web-interface.md -- CLI entry point for web interface
//
// Package web provides the ze web subcommand.
package web

import (
	"context"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/web"
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
	listen := fs.String("listen", "127.0.0.1:8443", "Listen address (host:port)")

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

	srv, err := web.NewWebServer(web.WebConfig{
		ListenAddr: *listen,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if err := srv.ListenAndServe(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze web [options]

Start the Ze web interface.

Options:
  --cert <path>    TLS certificate PEM file
  --key <path>     TLS private key PEM file (required with --cert)
  --listen <addr>  Listen address (default: 127.0.0.1:8443)

When --cert is not provided, a self-signed certificate is generated automatically.

Examples:
  ze web                                       Start with auto-generated cert
  ze web --listen 0.0.0.0:8443                 Listen on all interfaces
  ze web --cert server.pem --key server-key.pem  Use provided certificate
`)
}
