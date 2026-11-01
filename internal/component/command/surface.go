// Design: docs/architecture/api/commands.md — where a command is served
// Related: local.go — the local registrations the predicate classifies
// Related: local_data.go — the local-data registrations the predicate classifies
//
// surface.go answers one question for every command: on which operator surface
// must it be offered. Every surface producer (the SSH tree, the web admin tree,
// the web terminal completer, the MCP, REST and gRPC command list, the `ze cli`
// tree, shell completion) and the parity check read this answer, so no surface
// keeps its own list of what belongs on it.

package command

import (
	"fmt"
	"strconv"
)

// Surface names one operator surface that lists commands. The zero value is
// SurfaceUnspecified and is never a valid surface.
type Surface uint8

// The surfaces that list commands. gNMI is not one of them: it serves the YANG
// config tree and no command.
const (
	SurfaceUnspecified   Surface = iota
	SurfaceSSH                   // SSH session completion and help
	SurfaceWebAdmin              // the web admin console nav and forms
	SurfaceWebCompletion         // the web terminal's tab completion
	SurfaceMCP                   // MCP tools/list
	SurfaceREST                  // the REST command list
	SurfaceGRPC                  // the gRPC command list
	SurfaceCLI                   // `ze cli` interactive completion
	SurfaceShell                 // shell completion, `ze completion words`
)

// Surfaces returns every valid surface, in declaration order.
func Surfaces() []Surface {
	return []Surface{
		SurfaceSSH, SurfaceWebAdmin, SurfaceWebCompletion, SurfaceMCP,
		SurfaceREST, SurfaceGRPC, SurfaceCLI, SurfaceShell,
	}
}

// String names the surface for a check's failure line.
func (s Surface) String() string {
	switch s {
	case SurfaceUnspecified:
		return "unspecified"
	case SurfaceSSH:
		return "ssh"
	case SurfaceWebAdmin:
		return "web-admin"
	case SurfaceWebCompletion:
		return "web-completion"
	case SurfaceMCP:
		return "mcp"
	case SurfaceREST:
		return "rest"
	case SurfaceGRPC:
		return "grpc"
	case SurfaceCLI:
		return "ze-cli"
	case SurfaceShell:
		return "shell"
	}
	return "surface(" + strconv.Itoa(int(s)) + ")"
}

// RegistrationKind names the registry a command is declared in. The zero value
// is KindUnspecified and is never a valid kind.
type RegistrationKind uint8

// The registration kinds, one per command source.
const (
	KindUnspecified RegistrationKind = iota
	KindBuiltin                      // a RegisterRPCs entry under its YANG `ze:command` path
	KindLocal                        // a MustRegisterLocal or MustRegisterLocalMeta handler
	KindLocalData                    // a MustRegisterLocalData handler
	KindPlugin                       // a plugin's Stage 1 command declaration with no YANG node
	KindView                         // a CLI live view with no daemon RPC: a TUI-only command
)

// String names the kind for a check's failure line.
func (k RegistrationKind) String() string {
	switch k {
	case KindUnspecified:
		return "unspecified"
	case KindBuiltin:
		return "builtin"
	case KindLocal:
		return "local"
	case KindLocalData:
		return "local-data"
	case KindPlugin:
		return "plugin"
	case KindView:
		return "view"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// CommandRegistration is one command as its registry declares it.
type CommandRegistration struct {
	Path   string
	Kind   RegistrationKind
	Hidden bool // a plugin command declared Hidden: it runs when typed in full and is listed nowhere
}

// OffersCommand answers whether surface s must offer reg.
//
// Every builtin, local and local-data command is served by the daemon, so
// every surface offers it. A hidden command is offered nowhere. A TUI-only
// view runs only inside the interactive CLI model, so only the SSH session and
// `ze cli` offer it. A plugin command with no YANG node exists only in a
// running daemon's registry, so every daemon surface offers it, and shell
// completion, which answers at tab time with no daemon to ask, does not.
func OffersCommand(reg CommandRegistration, s Surface) (bool, error) {
	if s == SurfaceUnspecified {
		return false, fmt.Errorf("command %q: the surface is unspecified", reg.Path)
	}
	if s > SurfaceShell {
		return false, fmt.Errorf("command %q: no answer for surface %s", reg.Path, s)
	}
	if reg.Hidden {
		return false, nil
	}
	switch reg.Kind {
	case KindBuiltin, KindLocal, KindLocalData:
		return true, nil
	case KindPlugin:
		return s != SurfaceShell, nil
	case KindView:
		return s == SurfaceSSH || s == SurfaceCLI, nil
	case KindUnspecified:
		return false, fmt.Errorf("command %q: registration kind is unspecified", reg.Path)
	}
	return false, fmt.Errorf("command %q: no answer for registration kind %s", reg.Path, reg.Kind)
}
