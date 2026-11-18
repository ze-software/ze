// Design: docs/plugin-development/schema.md -- an external plugin brings its own YANG module
// Related: ../fixture/plugin_fixture_external_yang.go -- the test plugin that declares these modules
// Related: ../../component/config/cli/validate_corpus_test.go -- the config sweep publishes the accepted ones

// Package externalyang declares the YANG modules the external-yang test
// plugin can bring (internal/test/fixture, `le test fixture
// plugin/external-yang <variant>`).
//
// The modules are declared once, here, so both readers hold the same text:
// the plugin, which sends one at Stage 1, and the repository config sweep,
// which runs no plugin and publishes the accepted ones to judge the configs
// of the external-yang-* functional tests in full. The sweep cannot import
// the plugin's package, which depends on the package the sweep tests.
package externalyang

import (
	"slices"
	"strings"
)

// Module is one YANG module the test plugin can declare.
type Module struct {
	// Name is the module name the declaration states beside the text.
	Name string
	// Namespace is the namespace the declaration states beside the text.
	Namespace string
	// Text is the YANG text of the module.
	Text string
	// Refused says the daemon is expected to refuse the module: its defect is
	// what its test watches, so no reader joins it to a schema.
	Refused bool
}

// The valid module: its name ends in -conf, so its top-level container joins
// the config schema. interval carries a range and target is mandatory, so a
// test can watch the loader's own checks apply to a module the plugin brought.
// The container has a presence statement so a config with no acme-monitor
// block does not miss the mandatory leaf.
const validText = `module acme-monitor-conf {
    namespace "urn:acme:monitor:conf";
    prefix acme;

    description "Configuration of the acme monitor, an external plugin.";

    revision 2026-11-16 {
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

// RedeclaredText is the module text the redeclaring variant brings on its
// second start: the valid module with one more leaf, so its text differs from
// the published one.
const RedeclaredText = `module acme-monitor-conf {
    namespace "urn:acme:monitor:conf";
    prefix acme;

    description "Configuration of the acme monitor, an external plugin.";

    revision 2026-11-16 {
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

        leaf timeout {
            type uint8 {
                range "1..60";
            }
            description "Seconds before a probe is given up.";
        }
    }
}
`

// A module that does not compile: the container is never closed.
const invalidText = `module acme-broken-conf {
    namespace "urn:acme:broken:conf";
    prefix broken;

    container acme-broken {
        leaf count {
            type uint8;
        }
}
`

// A module named like the compiled-in module that defines the plugin block.
const clashText = `module ze-plugin-conf {
    namespace "urn:acme:clash:conf";
    prefix clash;

    container acme-clash {
        description "Settings under a module name the daemon already holds.";

        leaf enabled {
            type boolean;
            description "Whether the feature runs.";
        }
    }
}
`

// A module that reaches the compiled-in container service by augment, the
// sanctioned way to share a container. ze-dns-resolver-conf is the one
// always-compiled module that defines service.
const augmentText = `module acme-chrony-conf {
    namespace "urn:acme:chrony:conf";
    prefix chrony;

    import ze-dns-resolver-conf {
        prefix dnsresolver;
    }

    description "Configuration of the acme chrony service, an external plugin.";

    revision 2026-11-16 {
        description "Initial revision";
    }

    augment "/dnsresolver:service" {
        container chrony {
            presence "The chrony service is configured";
            description "Settings of the chrony service.";

            leaf server {
                type string {
                    length "1..253";
                }
                description "The time server chrony follows.";
            }
        }
    }
}
`

// Valid answers the module with a config root of its own, acme-monitor.
func Valid() Module {
	return Module{Name: "acme-monitor-conf", Namespace: "urn:acme:monitor:conf", Text: validText}
}

// Invalid answers the module that does not compile.
func Invalid() Module {
	return Module{Name: "acme-broken-conf", Namespace: "urn:acme:broken:conf", Text: invalidText, Refused: true}
}

// Clash answers the module named like a module the daemon compiles in.
func Clash() Module {
	return Module{Name: "ze-plugin-conf", Namespace: "urn:acme:clash:conf", Text: clashText, Refused: true}
}

// Augment answers the module that adds chrony to the compiled-in service
// container.
func Augment() Module {
	return Module{Name: "acme-chrony-conf", Namespace: "urn:acme:chrony:conf", Text: augmentText}
}

// commandsText is the command module the plugin brings beside Valid: one
// command with no argument, and one whose argument is a string of 1 to 16
// characters, so a test can watch the daemon judge a value before it asks
// the plugin. Its verb container states no help text: the verb is shared,
// and its help is the one ze holds.
const commandsText = `module acme-monitor-cmd {
    namespace "urn:acme:monitor:cmd";
    prefix acmecmd;
    import ze-extensions { prefix ze; }

    description "Commands of the acme monitor, an external plugin.";

    container show {
        config false;

        container acme-monitor {
            config false;
            ze:help "Inspect the acme monitor.";
            description "What the acme monitor polls and whether it is polling.";

            container status {
                config false;
                ze:command "acmecmd:show-status";
                ze:help "Show whether the acme monitor is polling.";
                description "Answers the state of the poller, from the plugin.";
            }

            container target {
                config false;
                ze:command "acmecmd:show-target";
                ze:rpc "acme-monitor-api:show-target";
                ze:help "Show one target the acme monitor polls.";
                description "Answers what the plugin holds for the target it is given.";

                leaf name {
                    type string {
                        length "1..16";
                    }
                    mandatory true;
                    ze:help "Name of the target.";
                    description "The target name, from 1 to 16 characters.";
                }
            }
        }
    }
}
`

// apiText is the rpc module the plugin brings beside Commands, as a
// compiled-in plugin pairs a -cmd module with an -api one: the target command
// points at its rpc with ze:rpc, and the rpc input is the parameter list the
// API surfaces publish for the command.
const apiText = `module acme-monitor-api {
    namespace "urn:acme:monitor:api";
    prefix acmeapi;
    import ze-extensions { prefix ze; }

    description "The rpcs of the acme monitor commands, an external plugin.";

    rpc show-target {
        ze:help "Show one target the acme monitor polls.";
        description "Answers what the plugin holds for the target it is given.";
        input {
            leaf name {
                type string {
                    length "1..16";
                }
                mandatory true;
                ze:help "Name of the target.";
                description "The target name, from 1 to 16 characters.";
            }
        }
    }
}
`

// API answers the rpc module the plugin brings beside Commands.
func API() Module {
	return Module{Name: "acme-monitor-api", Namespace: "urn:acme:monitor:api", Text: apiText}
}

// CommandStatus and CommandTarget are the commands Commands declares, as the
// plugin declares them at Stage 1.
const (
	CommandStatus = "show acme-monitor status"
	CommandTarget = "show acme-monitor target"
)

// DeclaredSummary and DeclaredExplanation are the two help texts the plugin
// states for each of its commands in its Stage 1 declaration. They differ from
// every text commandsText declares, so a surface that shows one of them shows
// the declaration where the YANG should win.
const (
	DeclaredSummary     = "Declared at Stage 1, not in the YANG."
	DeclaredExplanation = "The Stage 1 explanation, which the YANG node overrides."
)

// The texts commandsText declares, which a surface shows for the plugin's
// commands: the ze:help summary of each command and of the argument leaf, and
// the description of the status command.
const (
	StatusSummary     = "Show whether the acme monitor is polling."
	StatusExplanation = "Answers the state of the poller, from the plugin."
	TargetSummary     = "Show one target the acme monitor polls."
	ArgumentSummary   = "Name of the target."
	ArgumentText      = "The target name, from 1 to 16 characters."
)

// Commands answers the command module the plugin brings beside Valid.
func Commands() Module {
	return Module{Name: "acme-monitor-cmd", Namespace: "urn:acme:monitor:cmd", Text: commandsText}
}

// CommandsNarrowed answers Commands with the bound of the target name cut
// from 1..16 to 1..8 characters: the module a restarted plugin brings in
// place of Commands, so a test can watch the daemon keep the bound it started
// with.
func CommandsNarrowed() Module {
	text := strings.ReplaceAll(commandsText, `length "1..16";`, `length "1..8";`)
	text = strings.ReplaceAll(text, "from 1 to 16 characters", "from 1 to 8 characters")
	return Module{Name: "acme-monitor-cmd", Namespace: "urn:acme:monitor:cmd", Text: text}
}

// CommandClashPath is the command the clashing command module declares: a
// path the compiled-in module ze-cli-show-cmd already declares.
const CommandClashPath = "show version"

// commandClashText is a command module that declares `show version`, a
// command compiled into ze, so the join refuses it.
const commandClashText = `module acme-clash-cmd {
    namespace "urn:acme:clash:cmd";
    prefix acmeclash;
    import ze-extensions { prefix ze; }

    description "A command module that declares a command ze already holds.";

    container show {
        config false;

        container version {
            config false;
            ze:command "acmeclash:show-version";
            ze:help "Show the version of the acme plugin.";
            description "Answers the version of the plugin, under a path ze already holds.";
        }
    }
}
`

// CommandClash answers the command module that declares CommandClashPath.
func CommandClash() Module {
	return Module{Name: "acme-clash-cmd", Namespace: "urn:acme:clash:cmd", Text: commandClashText, Refused: true}
}

// Accepted answers the config modules the daemon is expected to accept,
// sorted by name: every config module of this package that is not Refused.
// Commands is not one of them: it adds no config, so the config sweep that
// reads this list has no use for it. RedeclaredText is not one of them: it is
// a second text of Valid's module, which a process joins once.
func Accepted() []Module {
	var accepted []Module
	for _, module := range []Module{Valid(), Invalid(), Clash(), Augment()} {
		if module.Refused {
			continue
		}
		accepted = append(accepted, module)
	}
	slices.SortFunc(accepted, func(a, b Module) int { return strings.Compare(a.Name, b.Name) })
	return accepted
}
