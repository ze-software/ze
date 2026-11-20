// Design: docs/plugin-development/commands.md — an external plugin defines its commands in YANG
// Related: ../python/acme_monitor.py — the same plugin in Python, with no Go SDK
// Related: ../../../internal/test/externalyang/externalyang.go — the test module whose config leaves acme-monitor-conf mirrors
//
// Example external ze plugin, written on the public Go SDK (pkg/plugin/sdk).
//
// It is the Go twin of the Python example: the same three YANG modules, the
// same config section and the same two commands. Ze reaches both the same way,
// so an author picks the language and nothing else changes.
//
// Its config module, acme-monitor-conf, has the leaves of the module of the
// same name that ze's tests declare for their own acme monitor plugin
// (internal/test/externalyang/externalyang.go, validText): the same names,
// types, bounds, mandatory leaf and presence, with no default. The
// repository config sweep judges every config under test/ with that module,
// so a config written for this example is valid there too. Change both
// together.
//
// Configuration:
//
//	acme-monitor {
//	    target web.example.net
//	    interval 3
//	}
//
// Commands:
//
//	show acme-monitor status
//	show acme-monitor target <name>
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// confYANG adds the config section acme-monitor. The module name ends in
// -conf, so its container joins the config schema. The container has a
// presence statement, so a config with no acme-monitor section does not miss
// the mandatory target.
const confYANG = `module acme-monitor-conf {
    namespace "urn:acme:monitor:conf";
    prefix acme;

    description "Configuration of the acme monitor, an external plugin.";

    revision 2026-11-17 {
        description "Initial revision";
    }

    container acme-monitor {
        presence "The acme monitor is configured";
        description "Settings of the acme monitor.";

        leaf target {
            type string {
                length "1..64";
            }
            mandatory true;
            description "The host the monitor probes.";
        }

        leaf interval {
            type uint8 {
                range "1..10";
            }
            description "Seconds between two probes.";
        }
    }
}
`

// cmdYANG adds the two commands. A command is a config false container that
// holds ze:command, under a verb container; its leaves are its arguments, and
// ze checks a value against the leaf's type before it asks the plugin.
const cmdYANG = `module acme-monitor-cmd {
    namespace "urn:acme:monitor:cmd";
    prefix acmecmd;
    import ze-extensions { prefix ze; }

    description "Commands of the acme monitor, an external plugin.";

    container show {
        config false;

        container acme-monitor {
            config false;
            ze:help "Inspect the acme monitor.";

            container status {
                config false;
                ze:command "acme-monitor:show-status";
                ze:help "Show the target and the probe interval.";
            }

            container target {
                config false;
                ze:command "acme-monitor:show-target";
                ze:rpc "acme-monitor-api:show-target";
                ze:help "Show the configured target of the acme monitor.";

                leaf name {
                    type string {
                        length "1..16";
                    }
                    mandatory true;
                    ze:help "Name of the target.";
                }
            }
        }
    }
}
`

// apiYANG holds the rpc of show acme-monitor target: its input is the
// parameter list MCP, REST and gRPC publish for the command.
const apiYANG = `module acme-monitor-api {
    namespace "urn:acme:monitor:api";
    prefix acmeapi;
    import ze-extensions { prefix ze; }

    description "The rpcs of the acme monitor commands, an external plugin.";

    rpc show-target {
        ze:help "Show the configured target of the acme monitor.";
        input {
            leaf name {
                type string {
                    length "1..16";
                }
                mandatory true;
                ze:help "Name of the target.";
            }
        }
    }
}
`

// The plugin's name, its config root, and the commands its -cmd module
// defines. A command name is the path of the container that holds its
// ze:command.
const (
	pluginName    = "acme-monitor"
	configRoot    = "acme-monitor"
	commandStatus = "show acme-monitor status"
	commandTarget = "show acme-monitor target"
)

// intervalUnset is how the answers write an interval the config does not
// state: the module has no default, so the plugin invents none.
const intervalUnset = "not set"

// declaration is what the plugin brings. It is a pure value: query mode
// writes it and starts nothing. Listing the commands is what makes ze ask
// this plugin for them.
func declaration() *sdk.Registration {
	return &sdk.Registration{
		Schemas: []sdk.SchemaDecl{
			{Module: "acme-monitor-conf", Namespace: "urn:acme:monitor:conf", YANGText: confYANG},
			{Module: "acme-monitor-cmd", Namespace: "urn:acme:monitor:cmd", YANGText: cmdYANG},
			{Module: "acme-monitor-api", Namespace: "urn:acme:monitor:api", YANGText: apiYANG},
		},
		WantsConfig: []string{configRoot},
		Commands: []sdk.CommandDecl{
			{Name: commandStatus},
			{Name: commandTarget},
		},
	}
}

// monitorConfig is the acme-monitor section, every value as a string. An
// empty Target is a section that is absent: the mandatory leaf is always set
// in a section ze delivered.
type monitorConfig struct {
	Target   string      `json:"target"`
	Interval json.Number `json:"interval"`
}

// intervalText answers the interval as the answers and the log write it.
func (cfg monitorConfig) intervalText() string {
	if cfg.Interval == "" {
		return intervalUnset
	}
	return cfg.Interval.String()
}

// parseMonitor reads the section from one delivery and refuses a target
// written as a URL. It changes no state, because a refused verify runs it too.
func parseMonitor(sections []sdk.ConfigSection) (monitorConfig, bool, error) {
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		var data struct {
			Monitor monitorConfig `json:"acme-monitor"`
		}
		if err := json.Unmarshal([]byte(section.Data), &data); err != nil {
			return monitorConfig{}, true, fmt.Errorf("acme-monitor config: %w", err)
		}
		if _, rest, url := strings.Cut(data.Monitor.Target, "://"); url {
			host, _, _ := strings.Cut(rest, "/")
			return monitorConfig{}, true, fmt.Errorf(
				"acme-monitor target %s is a URL, and the monitor probes a host name\n"+
					"fix: set acme-monitor target %s", data.Monitor.Target, host)
		}
		return data.Monitor, true, nil
	}
	return monitorConfig{}, false, nil
}

// monitor holds the running config the commands read. Safe for concurrent
// use.
type monitor struct {
	mu  sync.Mutex
	cfg monitorConfig
}

// install makes cfg the running config. It cannot fail, so the running config
// is never left half installed.
func (m *monitor) install(cfg monitorConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	slog.Info("acme-monitor: applied", "target", cfg.Target, "interval", cfg.intervalText())
	return nil
}

// command answers the two commands. Ze checked the target name against the
// leaf of acme-monitor-cmd before it asked, so args holds that one value.
func (m *monitor) command(_, command string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch command {
	case commandStatus:
		if m.cfg.Target == "" {
			return "", nil, errors.New("acme-monitor is not configured, so it probes nothing\n" +
				"fix: set acme-monitor target <host>")
		}
		return "done", map[string]string{"target": m.cfg.Target, "interval": m.cfg.intervalText()}, nil
	case commandTarget:
		if len(args) == 0 {
			return "", nil, errors.New("show acme-monitor target names no target\nfix: show acme-monitor target <name>")
		}
		name := args[0]
		if name != m.cfg.Target {
			configured := m.cfg.Target
			if configured == "" {
				configured = "none"
			}
			return "", nil, fmt.Errorf("acme-monitor has no target %s (configured: %s)\nfix: show acme-monitor status",
				name, configured)
		}
		return "done", map[string]string{"target": name, "interval": m.cfg.intervalText()}, nil
	}
	return "", nil, fmt.Errorf("acme-monitor does not serve %q", command)
}

// start is the live start: connect, register the handlers, run. Query mode
// never calls it.
func start() int {
	p, err := sdk.NewFromEnv(pluginName)
	if err != nil {
		slog.Error("acme-monitor: cannot connect to ze", "error", err)
		return 1
	}
	defer p.Close() //nolint:errcheck // the run result carries the useful error

	m := &monitor{}
	sdk.NewConfigTx(monitorConfig{}, parseMonitor, m.install).Register(p)
	p.OnExecuteCommand(m.command)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := p.Run(ctx, declaration()); err != nil {
		slog.Error("acme-monitor: stopped", "error", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(sdk.RunOrDeclare(declaration(), start))
}
