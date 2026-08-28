package cli

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

func cmdStaticHTTP(args []string) int {
	flags := flag.NewFlagSet("static-http", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	bind := flags.String("bind", "127.0.0.1:8080", "listen address")
	directory := flags.String("directory", ".", "directory to serve")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "static-http: unexpected positional arguments")
		return 2
	}
	listener, err := net.Listen("tcp", *bind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "static-http: listen: %v\n", err)
		return 1
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(*directory)), ReadHeaderTimeout: 5 * time.Second}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "static-http: serve: %v\n", err)
		return 1
	}
	return 0
}
