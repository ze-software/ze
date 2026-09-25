package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// CmdStaticHTTP is the harness command `le test httpd`, registered by
// internal/le/test/statichttp. It answers the process exit code.
func CmdStaticHTTP(args []string) int {
	flags := flag.NewFlagSet("httpd", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	bind := flags.String("bind", "127.0.0.1:8080", "listen address")
	directory := flags.String("directory", ".", "directory to serve")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "httpd: unexpected positional arguments")
		return 2
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", *bind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "httpd: listen: %v\n", err)
		return 1
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(*directory)), ReadHeaderTimeout: 5 * time.Second}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "httpd: serve: %v\n", err)
		return 1
	}
	return 0
}
