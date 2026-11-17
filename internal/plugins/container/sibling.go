// Design: docs/architecture/container/runtime.md -- the sibling seam: config, apply, engine calls, commands and doctor checks
// Related: participant.go -- the start and end calls a sibling also takes part in
// Related: sibling_engine.go -- the engine client a sibling is handed
// Related: sibling_install.go -- the install that runs each sibling's apply before the supervisor's
// Related: config.go -- the parse that runs each sibling's parse
// Related: container.go -- the engine entry, which hands the client and dispatches the commands

package container

import (
	"context"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A sibling is a package of this plugin other than the runtime: the network
// package is one. Beside the participant calls (participant.go) it takes part
// in the config transaction, reaches the engine through a narrow client, and
// answers commands and doctor checks the plugin declares for it. It registers
// all of them from its own init(), and this package never imports it.
//
// Go runs this package's init before any sibling's, because a sibling imports
// this package. So the registration this package hands the plugin registry
// (register.go) is built before any sibling registered: a field of it that a
// sibling would extend is extended through the registry's own late call
// (RegisterSiblingConfigRead), or is read only when the engine entry runs.

// siblingRegistry holds what the siblings registered. The engine entry uses
// registeredSiblings; a test builds one of its own, so a fake sibling never
// reaches another test. It is written only from package init functions,
// before any reader runs, and only read after: it needs no lock.
type siblingRegistry struct {
	configs   []configSibling
	commands  []SiblingCommand
	checks    []SiblingDoctorCheck
	receivers []func(EngineClient)
}

// registeredSiblings is what the siblings of this build registered.
var registeredSiblings siblingRegistry

// configSibling is one sibling's part of the config transaction: its typed
// parse and apply, wrapped so this package holds them without knowing the
// sibling's config type. unconfigured is the sibling's zero config, what it
// runs before any config was installed.
type configSibling struct {
	name         string
	parse        func(sections []sdk.ConfigSection) (any, error)
	apply        func(ctx context.Context, cfg any) error
	unconfigured any
}

// siblingValue is one sibling's parsed config inside a containerConfig. The
// config transaction compares whole containerConfig values with
// reflect.DeepEqual, so a sibling's config type holds no func and no channel,
// or every commit installs it again.
type siblingValue struct {
	name  string
	value any
}

// RegisterConfigSibling adds a sibling to the plugin's config transaction.
// It MUST be called from a package init function.
//
// parse reads the sibling's own nodes from the sections the plugin receives:
// the container root, and each root the sibling added with
// RegisterSiblingConfigRead. It runs after the runtime's own parse accepted
// the container root, at `ze config validate`, at a reload's verify and at the
// boot configure, and only when the delivery carries the container root. It
// MUST NOT change any state. An error refuses the config: it reaches the
// operator as it is, so it MUST be an opererr.Error.
//
// apply makes cfg the sibling's running config. It is called with the
// engine reachable, never before the plugin started: for the boot config when
// the engine reports the plugin started, before any container starts; for a
// reload, inside the transaction's apply and its rollback. It runs before the
// supervisor's own apply, so what it makes exists before a container of the
// new config starts, while the containers of the old config still run: it
// MUST keep what a running container uses until that container's ended call.
// It is called at every install, an unchanged sibling config included, so it
// MUST be idempotent. An error refuses the commit: the siblings that applied
// before it get their previous config back, the supervisor's apply is not
// made, and the error reaches the operator as it is. Its context ends after
// participantCallTimeout.
//
// T MUST be a concrete type, not an interface, and its zero value MUST be the
// sibling's config with nothing configured: it is applied at the start when
// no config was delivered, and for a rollback to the config before the first
// install.
//
// A nil func, an empty name and a name registered twice are Ze defects and
// panic.
func RegisterConfigSibling[T any](name string, parse func([]sdk.ConfigSection) (T, error), apply func(context.Context, T) error) {
	registeredSiblings.addConfig(newConfigSibling(name, parse, apply))
}

// newConfigSibling wraps a sibling's typed parse and apply.
func newConfigSibling[T any](name string, parse func([]sdk.ConfigSection) (T, error), apply func(context.Context, T) error) configSibling {
	if parse == nil {
		panic("BUG: container config sibling " + name + " registered with a nil parse")
	}
	if apply == nil {
		panic("BUG: container config sibling " + name + " registered with a nil apply")
	}
	var unconfigured T
	return configSibling{
		name: name,
		parse: func(sections []sdk.ConfigSection) (any, error) {
			return parse(sections)
		},
		apply: func(ctx context.Context, cfg any) error {
			typed, ok := cfg.(T)
			if !ok {
				panic("BUG: container config sibling " + name + " was handed a config its parse did not produce")
			}
			return apply(ctx, typed)
		},
		unconfigured: unconfigured,
	}
}

func (set *siblingRegistry) addConfig(sibling configSibling) {
	if sibling.name == "" {
		panic("BUG: container config sibling registered with an empty name")
	}
	for _, registered := range set.configs {
		if registered.name == sibling.name {
			panic("BUG: container config sibling " + sibling.name + " registered twice")
		}
	}
	set.configs = append(set.configs, sibling)
}

// valueIn answers the sibling's config inside cfg, or its unconfigured config
// when cfg holds none: the config before the first install holds no sibling.
func (sibling *configSibling) valueIn(cfg containerConfig) any {
	for _, held := range cfg.siblings {
		if held.name == sibling.name {
			return held.value
		}
	}
	return sibling.unconfigured
}

// RegisterSiblingConfigRead makes the plugin receive the config root root
// beside its own, so a sibling's parse reads it: the network package reads
// the interface and firewall roots. A read root never loads the plugin. It
// MUST be called from a package init function. A refusal of the registry is a
// Ze defect and panics.
func RegisterSiblingConfigRead(root string) {
	if err := registry.AddConfigRead(pluginName, root); err != nil {
		panic("BUG: container config read " + root + ": " + err.Error())
	}
}

// SiblingCommand is one command a sibling answers in the plugin's process.
type SiblingCommand struct {
	// Decl is the command's declaration, sent at Stage 1 with the plugin's own.
	Decl sdk.CommandDecl
	// WireMethod is the method the command's YANG node binds with
	// ze:command. The plugin registers its proxy, which forwards the command
	// to the plugin's process, internal or forked.
	WireMethod string
	// Handle answers the command. It runs on the SDK's command goroutine, one
	// command at a time, and its error reaches the operator as it is, so it
	// MUST be an opererr.Error. It MUST return when ctx ends.
	Handle func(ctx context.Context, args []string) (any, error)
}

// RegisterSiblingCommand adds a command to the plugin's declarations and
// dispatch, and registers its proxy. It MUST be called from a package init
// function. A field left empty and a command another registration already
// answers are Ze defects and panic.
func RegisterSiblingCommand(command SiblingCommand) {
	registeredSiblings.addCommand(command)
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod:    command.WireMethod,
		Handler:       forwardToPlugin(command.Decl.Name),
		PluginCommand: command.Decl.Name,
	})
}

func (set *siblingRegistry) addCommand(command SiblingCommand) {
	name := command.Decl.Name
	if name == "" {
		panic("BUG: container sibling command registered with an empty name")
	}
	if command.WireMethod == "" {
		panic("BUG: container sibling command " + name + " registered with no wire method")
	}
	if command.Handle == nil {
		panic("BUG: container sibling command " + name + " registered with no handler")
	}
	if name == cmdRequestRestart {
		panic("BUG: container sibling command " + name + " is the runtime's own")
	}
	if _, image := imageCommandHandlers[name]; image {
		panic("BUG: container sibling command " + name + " is an image command")
	}
	if _, known := set.commandHandler(name); known {
		panic("BUG: container sibling command " + name + " registered twice")
	}
	set.commands = append(set.commands, command)
}

// commandHandler answers the handler of the sibling command name.
func (set *siblingRegistry) commandHandler(name string) (func(context.Context, []string) (any, error), bool) {
	for i := range set.commands {
		if set.commands[i].Decl.Name == name {
			return set.commands[i].Handle, true
		}
	}
	return nil, false
}

// commandDecls answers the siblings' command declarations, in registration
// order.
func (set *siblingRegistry) commandDecls() []sdk.CommandDecl {
	decls := make([]sdk.CommandDecl, 0, len(set.commands))
	for i := range set.commands {
		decls = append(decls, set.commands[i].Decl)
	}
	return decls
}

// SiblingDoctorCheck is one doctor check a sibling answers from its own
// records.
type SiblingDoctorCheck struct {
	// Decl is the check's declaration, sent at Stage 1 with the runtime's
	// checks. Each code it names MUST be registered with diagnostic.Register
	// from the sibling's own init, before this registration.
	Decl rpc.DoctorCheckDecl
	// Answer answers show doctor's request for the check: one finding, with
	// its fix, for each problem the sibling holds. It runs on the SDK's
	// doctor goroutine, only reads, and MUST NOT block: it makes no engine
	// call.
	Answer func() ([]rpc.DoctorCheckDiagnostic, error)
}

// RegisterSiblingDoctorCheck adds a doctor check to the plugin's
// declarations and its doctor answer. It MUST be called from a package init
// function. A check with no name, no code or no answer, and a name another
// check of the plugin holds, are Ze defects and panic.
func RegisterSiblingDoctorCheck(check SiblingDoctorCheck) {
	registeredSiblings.addDoctorCheck(check)
}

func (set *siblingRegistry) addDoctorCheck(check SiblingDoctorCheck) {
	name := check.Decl.Name
	if name == "" {
		panic("BUG: container sibling doctor check registered with an empty name")
	}
	if len(check.Decl.Codes) == 0 {
		panic("BUG: container sibling doctor check " + name + " declares no code")
	}
	if check.Answer == nil {
		panic("BUG: container sibling doctor check " + name + " registered with no answer")
	}
	if isRuntimeOrStoreCheck(name) {
		panic("BUG: container sibling doctor check " + name + " is a check of the runtime or the store")
	}
	if _, known := set.doctorCheck(name); known {
		panic("BUG: container sibling doctor check " + name + " registered twice")
	}
	set.checks = append(set.checks, check)
}

// isRuntimeOrStoreCheck answers whether name is a check this package declares
// itself.
func isRuntimeOrStoreCheck(name string) bool {
	for i := range runtimeDoctorChecks {
		if runtimeDoctorChecks[i].decl.Name == name {
			return true
		}
	}
	return storeDoctorCheckNamed(name) != nil
}

// doctorCheck answers the sibling check named name.
func (set *siblingRegistry) doctorCheck(name string) (*SiblingDoctorCheck, bool) {
	index := slices.IndexFunc(set.checks, func(check SiblingDoctorCheck) bool { return check.Decl.Name == name })
	if index < 0 {
		return nil, false
	}
	return &set.checks[index], true
}

// doctorCheckDecls answers the siblings' check declarations, in registration
// order.
func (set *siblingRegistry) doctorCheckDecls() []rpc.DoctorCheckDecl {
	decls := make([]rpc.DoctorCheckDecl, 0, len(set.checks))
	for i := range set.checks {
		decls = append(decls, set.checks[i].Decl)
	}
	return decls
}

// RegisterEngineClientReceiver asks for the plugin's engine client. receive
// is called once for each run of the engine entry, on the engine entry's
// goroutine, before the start-up sweep makes any ended call and before any
// other call of this seam. The client refuses every request until the engine
// reports the plugin started, so an ended call of the sweep that uses it gets
// a refusal at once, never a hang. receive MUST only store the client. It
// MUST be called from a package init function; a nil receive is a Ze defect
// and panics.
func RegisterEngineClientReceiver(receive func(EngineClient)) {
	registeredSiblings.addReceiver(receive)
}

func (set *siblingRegistry) addReceiver(receive func(EngineClient)) {
	if receive == nil {
		panic("BUG: container engine client receiver registered as nil")
	}
	set.receivers = append(set.receivers, receive)
}

// handEngine hands client to every receiver.
func (set *siblingRegistry) handEngine(client EngineClient) {
	for _, receive := range set.receivers {
		receive(client)
	}
}
