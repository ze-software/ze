package gokapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gokrazy/gokapi/ondeviceapi"
	"github.com/gokrazy/internal/config"
	"github.com/gokrazy/internal/updateflag"
)

// ConnectOnDevice is meant to be run from a program running on a gokrazy
// instance and establishes a connection to the gokrazy on-device API.
func ConnectOnDevice() (*ondeviceapi.Configuration, error) {
	const gokrazyUnixSocket = "/run/gokrazy-http.sock"

	pw, err := os.ReadFile("/etc/gokr-pw.txt")
	pwErr := err // handled below
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	cfg := ondeviceapi.NewConfiguration()
	auth := "gokrazy:" + strings.TrimSpace(string(pw))
	cfg.BasePath = "http://" + auth + "@unix"

	// Use the Unix domain socket if available.
	if conn, err := net.Dial("unix", gokrazyUnixSocket); err == nil {
		conn.Close()
		cfg.HTTPClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", gokrazyUnixSocket)
				},
			},
		}
		return cfg, nil
	}

	// Connecting to the Unix domain socket failed. Now we need a password, so
	// error out if /etc/gokr-pw.txt was not found.
	if pwErr != nil {
		return nil, pwErr
	}

	// Fallback to TCP.
	port, err := os.ReadFile("/etc/http-port.txt")
	if err != nil {
		return nil, err
	}
	cfg.BasePath = "http://" + auth + "@localhost:" + strings.TrimSpace(string(port))
	return cfg, nil
}

// ConnectRemotely is meant to be run from a program running on any machine,
// remotely connecting to the on-device API of a gokrazy instance.
func ConnectRemotely(cfg *config.Struct) (*ondeviceapi.Configuration, error) {
	update, err := cfg.Update.WithFallbackToHostSpecific(cfg.Hostname)
	if err != nil {
		return nil, err
	}

	if update.HTTPPort == "" {
		update.HTTPPort = "80"
	}

	if update.HTTPSPort == "" {
		update.HTTPSPort = "443"
	}

	schema := "http"
	if update.UseTLS == "self-signed" {
		schema = "https"
	}
	u, err := updateflag.Value{Update: "yes"}.BaseURL(update.HTTPPort, update.HTTPSPort, schema, update.Hostname, update.HTTPPassword)
	if err != nil {
		return nil, err
	}

	result := ondeviceapi.NewConfiguration()
	result.BasePath = strings.TrimSuffix(u.String(), "/")
	return result, nil
}
