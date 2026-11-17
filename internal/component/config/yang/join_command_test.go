package yang

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// joinBaseCommands stands for a compiled-in -cmd module: it declares the
// command `show system` under the verb every -cmd module shares.
const joinBaseCommands = `module ze-join-base-cmd {
    namespace "urn:ze:join-base-cmd";
    prefix basecmd;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container system {
            config false;
            ze:command "ze-join-base:show-system";
        }
    }
}`

// joinAcmeCommands is the -cmd module of plugin "acme"; %RPC% is replaced by
// the ze:rpc statement a test gives the target command, %STATUS% by the wire
// method of the status command, and %PATH% by the name of its container.
const joinAcmeCommands = `module acme-monitor-cmd {
    namespace "urn:acme:monitor:cmd";
    prefix acmecmd;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container %PATH% {
            config false;
            ze:command "%STATUS%";
        }
        container acme-monitor {
            config false;
            container target {
                config false;
                ze:command "acmecmd:show-target";
                %RPC%
                leaf name {
                    type string { length "1..16"; }
                    mandatory true;
                }
            }
        }
    }
}`

// joinAcmeAPI is the -api module of plugin "acme": the rpc the target command
// points at, whose input is the parameter list the API surfaces publish.
const joinAcmeAPI = `module acme-monitor-api {
    namespace "urn:acme:monitor:api";
    prefix acmeapi;
    rpc show-target {
        input {
            leaf name {
                type string { length "1..16"; }
                mandatory true;
            }
        }
    }
}`

// acmeCommandModules answers the -cmd and -api declarations of plugin
// "acme", with the status command at path under the wire method status and
// the target command carrying rpc.
func acmeCommandModules(path, status, rpc string) []ExternalModule {
	text := strings.NewReplacer("%PATH%", path, "%STATUS%", status, "%RPC%", rpc).Replace(joinAcmeCommands)
	return []ExternalModule{
		acmeModule("acme-monitor-cmd", "urn:acme:monitor:cmd", text),
		acmeModule("acme-monitor-api", "urn:acme:monitor:api", joinAcmeAPI),
	}
}

// acmeTargetRPC is the ze:rpc statement that names the rpc joinAcmeAPI defines.
const acmeTargetRPC = `ze:rpc "acme-monitor-api:show-target";`

// joinCompiledWithCommands answers joinCompiled with joinBaseCommands added.
func joinCompiledWithCommands(t *testing.T) *Resolved {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	for name, text := range map[string]string{
		"ze-join-base-conf.yang": joinBaseModule,
		"ze-join-base-cmd.yang":  joinBaseCommands,
	} {
		if err := loader.AddModuleFromText(name, text); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	compiled, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve the compiled-in fixture: %v", err)
	}
	return compiled
}

// TestJoinAcceptsCommandAndAPIModules: a plugin's -cmd module and the -api
// module its ze:rpc names are joined, and the joined set's command tree and
// published rpcs hold the plugin's commands as they hold a compiled-in one.
//
// VALIDATES: the command step of spec-plugin-external-yang-at-startup: an
// external -cmd module reaches BuildCommandTree with its wire method and its
// typed argument, and PublishedRPCs links its ze:rpc to the -api rpc.
// PREVENTS: the join refusing a command module as one that adds nothing.
func TestJoinAcceptsCommandAndAPIModules(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC), joinValidators())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	tree := BuildCommandTree(joined)
	target := command.FindNode(tree, []string{"show", "acme-monitor", "target"})
	if target == nil {
		t.Fatal("the joined command tree lacks show acme-monitor target")
	}
	if target.WireMethod != "acmecmd:show-target" {
		t.Errorf("show acme-monitor target has wire method %q, want acmecmd:show-target", target.WireMethod)
	}
	if len(target.ArgDefs) != 1 {
		t.Fatalf("show acme-monitor target carries %d argument definitions, want 1", len(target.ArgDefs))
	}
	if command.FindNode(tree, []string{"show", "system"}) == nil {
		t.Error("the joined command tree lost the compiled-in show system")
	}
	published, err := PublishedRPCs(joined)
	if err != nil {
		t.Fatalf("PublishedRPCs over the joined set: %v", err)
	}
	linked := false
	for _, rpc := range published.Commands {
		if rpc.WireMethod == "acmecmd:show-target" && rpc.Module == "acme-monitor-api" && len(rpc.Input) == 1 {
			linked = true
		}
	}
	if !linked {
		t.Errorf("PublishedRPCs does not link acmecmd:show-target to rpc acme-monitor-api:show-target with its input: %+v", published.Commands)
	}
}

// TestJoinRefusesCommandModuleWithNoCommand: a -cmd module that declares no
// ze:command adds nothing, and the refusal names the statement to add.
//
// VALIDATES: the reworded refusal of a module that adds nothing.
// PREVENTS: a command module joined with no command, whose author sees no
// command and no reason.
func TestJoinRefusesCommandModuleWithNoCommand(t *testing.T) {
	module := `module acme-idle-cmd {
    namespace "urn:acme:idle:cmd";
    prefix idle;
    container show {
        config false;
        container acme-idle { config false; }
    }
}`
	joined, err := Join(joinCompiled(t), []ExternalModule{acmeModule("acme-idle-cmd", "urn:acme:idle:cmd", module)}, joinValidators())
	requireJoinRefusal(t, joined, err, `plugin "acme"`, `"acme-idle-cmd"`, "adds nothing", "ze:command")
}

// TestJoinRefusesCommandPathAlreadyDeclared: a ze:command on a path where a
// compiled-in module already declares one is refused, naming that module.
//
// VALIDATES: the held-path refusal, with the plugin, the module and the path.
// PREVENTS: BuildCommandTree keeping the first ze:command in silence, so the
// plugin's command never runs.
func TestJoinRefusesCommandPathAlreadyDeclared(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("system", "acmecmd:show-status", acmeTargetRPC), joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `"acme-monitor-cmd"`, `command "show system"`, `module "ze-join-base-cmd", compiled into ze`)
}

// TestJoinRefusesWireMethodAlreadyDeclared: a wire method a compiled-in module
// already declares is refused.
//
// VALIDATES: the held wire method refusal, naming the method and its holder.
// PREVENTS: two commands answering under one wire method.
func TestJoinRefusesWireMethodAlreadyDeclared(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("acme-status", "ze-join-base:show-system", acmeTargetRPC), joinValidators())
	requireJoinRefusal(t, joined, err,
		`plugin "acme"`, `wire method "ze-join-base:show-system"`, `module "ze-join-base-cmd", compiled into ze`)
}

// TestJoinRefusesRPCTheModuleDoesNotDeclare: a ze:rpc naming an rpc the -api
// module does not declare is refused through PublishedRPCs, the check every
// compiled-in pointer obeys.
//
// VALIDATES: the joined set is judged by PublishedRPCs, and the refusal names
// the plugin and carries the pointer.
// PREVENTS: a command published with no parameters, or the API metadata
// failing at its first use after the daemon started.
func TestJoinRefusesRPCTheModuleDoesNotDeclare(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("acme-status", "acmecmd:show-status", `ze:rpc "acme-monitor-api:no-such-rpc";`), joinValidators())
	requireJoinRefusal(t, joined, err, `plugin "acme"`, "acme-monitor-api:no-such-rpc")
}

// TestJoinRefusesRPCInAModuleNobodyBrings: a ze:rpc naming a module neither
// the plugin nor ze holds is refused.
//
// VALIDATES: an unlinked pointer that only the external module adds is a
// refusal, naming the pointer.
// PREVENTS: a plugin's command whose parameters no module describes.
func TestJoinRefusesRPCInAModuleNobodyBrings(t *testing.T) {
	joined, err := Join(joinCompiledWithCommands(t),
		acmeCommandModules("acme-status", "acmecmd:show-status", `ze:rpc "acme-ghost-api:show-target";`), joinValidators())
	requireJoinRefusal(t, joined, err, `plugin "acme"`, "acme-ghost-api:show-target", "does not hold")
}

// TestPublishedCommandModuleRebuildsArgumentDefinitions: after a publish of a
// plugin's -cmd module, the definitions every dispatch route judges a command
// against (command.ValidateModelArgs) hold the plugin's argument, so a value
// its type refuses and a missing value are refused and a good one passes.
// Method: build the definitions first, so the publish has to rebuild them.
//
// VALIDATES: rebuildCommandArgDefs over the joined set reaches route R9.
// PREVENTS: definitions built before the publish answering the external
// command with no argument, which would pass every value to the plugin.
func TestPublishedCommandModuleRebuildsArgumentDefinitions(t *testing.T) {
	t.Cleanup(ResetPublishedModulesForTest())
	if _, err := commandArgDefs("show"); err != nil {
		t.Fatalf("build the definitions before the publish: %v", err)
	}
	modules := acmeCommandModules("acme-status", "acmecmd:show-status", acmeTargetRPC)
	if err := PublishExternalModules(modules, joinValidators()); err != nil {
		t.Fatalf("PublishExternalModules: %v", err)
	}
	const path = "show acme-monitor target"
	if _, err := command.ValidateModelArgs(path, []string{"this-name-is-far-too-long"}, nil); err == nil {
		t.Error("a name of 25 characters passed a leaf of length 1..16")
	}
	if _, err := command.ValidateModelArgs(path, nil, nil); err == nil {
		t.Error("the line with no name passed a mandatory leaf")
	}
	if _, err := command.ValidateModelArgs(path, []string{"probe-b"}, nil); err != nil {
		t.Errorf("a good name was refused: %v", err)
	}
}
