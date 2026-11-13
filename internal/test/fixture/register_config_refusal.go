// Design: docs/guide/api.md -- config sessions preserve schema-derived refusals.
// Related: plugin_fixture_config_refusal.go -- clients use TLS and a bearer token.

package fixture

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("config-refusal/tls", configRefusalTLS)
	Register("plugin/api-config-enum-refusal", restConfigRefusal, StopsDaemon)
	Register("plugin/grpc-config-enum-refusal", grpcConfigRefusal, StopsDaemon)
	Register("plugin/gnmi-set-enum-refusal", gnmiConfigRefusal, StopsDaemon)
}

func restConfigRefusal(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("plugin/api-config-enum-refusal needs a REST endpoint URL")
	}
	return Observe(ctx, "api-enum-refusal", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return plugin01APIConfigEnumRefusal(ctx, plugin, args[0])
	})
}

// configRefusalTLS exports the existing fixture certificate for a real daemon
// listener; its certificate is also the client's pinned trust anchor.
func configRefusalTLS(_ context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("config-refusal/tls needs certificate and key paths")
	}
	pair, err := managedSelfSignedCert()
	if err != nil {
		return err
	}
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		return err
	}
	if err := os.WriteFile(args[0], pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(args[1], pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600)
}

func grpcConfigRefusal(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("plugin/grpc-config-enum-refusal needs listener address and certificate path")
	}
	return Observe(ctx, "grpc-enum-refusal", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		return grpcConfigEnumRefusal(ctx, args[0], args[1])
	})
}

func gnmiConfigRefusal(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("plugin/gnmi-set-enum-refusal needs listener address and certificate path")
	}
	return Observe(ctx, "gnmi-enum-refusal", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		return gnmiSetEnumRefusal(ctx, plugin, args[0], args[1])
	})
}
