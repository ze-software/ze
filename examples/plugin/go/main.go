// Example ZeBGP plugin that monitors an external HTTP endpoint.
//
// This plugin demonstrates:
//   - YANG schema declaration
//   - Verify handler (validates config)
//   - Apply handler (applies config)
//   - Command handler (returns status)
//
// Configuration:
//
//	acme-monitor {
//	    endpoint "https://api.example.com/health";
//	    interval 60;
//	}
//
// Commands:
//
//	acme-monitor status
package main

import (
	"fmt"
	"net/url"
	"os"

	"codeberg.org/thomas-mangin/ze/pkg/plugin"
)

// YANG schema for the monitor plugin.
const yangSchema = `module acme-monitor {
    namespace "urn:acme:monitor";
    prefix acme;

    container acme-monitor {
        leaf endpoint {
            type string;
            mandatory true;
            description "HTTP endpoint to monitor";
        }
        leaf interval {
            type uint32 {
                range "10..3600";
            }
            default 60;
            description "Check interval in seconds";
        }
    }
}`

// State holds the current monitoring configuration.
type State struct {
	Endpoint string
	Interval uint32
}

var state State

func main() {
	p := plugin.New("acme-monitor")
	if p == nil {
		fmt.Fprintln(os.Stderr, "failed to create plugin")
		os.Exit(1)
	}

	// Set YANG schema and handler prefix
	if err := p.SetSchema(yangSchema, "acme-monitor"); err != nil {
		fmt.Fprintf(os.Stderr, "set schema: %v\n", err)
		os.Exit(1)
	}

	// Register verify handler - validates config before apply
	p.OnVerify("acme-monitor", func(ctx *plugin.VerifyContext) error {
		// For delete action, no validation needed
		if ctx.Action == "delete" {
			return nil
		}

		// Validate endpoint is a valid HTTPS URL
		// Note: In real code, parse ctx.Data as JSON
		if ctx.Data == "" {
			return fmt.Errorf("missing config data")
		}

		// Simplified validation - real plugin would parse JSON
		// and check the endpoint field
		return nil
	})

	// Register apply handler - applies validated config
	p.OnApply("acme-monitor", func(ctx *plugin.ApplyContext) error {
		switch ctx.Action {
		case "create", "modify":
			// In real plugin, parse ctx data and start/update monitoring
			state.Endpoint = "configured"
			state.Interval = 60
		case "delete":
			state.Endpoint = ""
			state.Interval = 0
		}
		return nil
	})

	// Register status command
	p.OnCommand("acme-monitor status", func(ctx *plugin.CommandContext) (any, error) {
		return map[string]any{
			"endpoint": state.Endpoint,
			"interval": state.Interval,
			"status":   getStatus(),
		}, nil
	})

	// Run the plugin protocol loop
	if err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "plugin error: %v\n", err)
		os.Exit(1)
	}
}

// getStatus returns current monitoring status.
func getStatus() string {
	if state.Endpoint == "" {
		return "not configured"
	}
	return "running"
}

// validateEndpoint checks if the endpoint is a valid HTTPS URL.
func validateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("endpoint must use HTTPS, got: %s", u.Scheme)
	}
	return nil
}
