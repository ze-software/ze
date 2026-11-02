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
	KindPlugin                       // a plugin's Stage 1 command declaration with no `ze:command` node of its own
	KindView                         // a CLI live view with no other registration
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

// CommandRegistration is one command as its registries declare it. Every field
// is read from a registration, never from the command's name.
type CommandRegistration struct {
	Path string
	Kind RegistrationKind
	// Hidden is a plugin command declared Hidden: it runs when typed in full
	// and is listed nowhere.
	Hidden bool
	// Streaming says a streaming handler (RegisterStreamingHandler) serves the
	// path, so a terminal surface runs it as a live stream.
	Streaming bool
	// StreamingOnly says the streaming registration declares no one-shot
	// answer (pluginserver.OneShotStreamingOnly): its stream is a sequence of
	// events, so a one-shot surface does not offer it. Set only with Streaming.
	StreamingOnly bool
	// TUIOnly says the command's view registration (cli.RegisterView) declares
	// that only the interactive CLI can render it.
	TUIOnly bool
	// DaemonPath is the YANG path a local command at an offline root declares
	// as its daemon form (registry.Meta.DaemonPath): the daemon surfaces offer
	// that path in this one's place, and shell completion offers this one.
	// Set only on a local or local-data registration.
	DaemonPath string
	// Modeled says a plugin command's path is a node of the shipped YANG
	// schema although no `ze:command` sits on it (`show vrrp interface`,
	// whose command node is `name` below it). Shell completion walks that
	// schema with no daemon, so it offers the path. Set only on a plugin
	// registration.
	Modeled bool
}

// Offer is what one surface owes one command. The zero value is
// OfferUnspecified and is never an answer.
type Offer uint8

// The offers a surface can owe a command.
const (
	OfferUnspecified Offer = iota
	OfferMust              // listed, and running it answers once
	OfferMustNot           // listed nowhere on this surface
	OfferStreaming         // listed, and running it streams live until the operator stops it
	OfferTUIOnly           // listed, and running it opens the interactive CLI's live view
)

// String names the offer for a check's failure line.
func (o Offer) String() string {
	switch o {
	case OfferUnspecified:
		return "unspecified"
	case OfferMust:
		return "must offer"
	case OfferMustNot:
		return "must not offer"
	case OfferStreaming:
		return "must offer, streaming"
	case OfferTUIOnly:
		return "must offer, TUI-only"
	}
	return "offer(" + strconv.Itoa(int(o)) + ")"
}

// Listed answers whether a surface owing o lists the command.
func (o Offer) Listed() bool {
	switch o {
	case OfferMust, OfferStreaming, OfferTUIOnly:
		return true
	case OfferUnspecified, OfferMustNot:
		return false
	}
	panic("BUG: Offer.Listed: offer outside the declared set")
}

// SurfaceAnswer is the predicate's answer for one command on one surface: the
// offer, and the reason a failure line prints beside it.
type SurfaceAnswer struct {
	Offer  Offer
	Reason string
}

// The reasons AnswerSurface gives. A check prints them, so each says why.
const (
	reasonHidden       = "declared hidden: it runs when typed in full and is listed nowhere"
	reasonServed       = "served by the daemon under its YANG path and locally by `ze <verb>`"
	reasonStreaming    = "a monitor command streams live on the local terminal, `ze cli` and SSH"
	reasonPluginDaemon = "a plugin command exists in a running daemon's registry, and every daemon surface reads it"
	reasonPluginShell  = "a plugin command exists only in a running daemon's registry, and shell completion answers with no daemon"
	reasonPluginSchema = "a plugin command whose path is a node of the shipped schema, which shell completion walks with no daemon"
	reasonStreamingOff = "declared streaming-only: its stream is a sequence of events, so a one-shot surface has no answer to give"
	reasonTUIOnly      = "declared TUI-only on its view: only the interactive CLI renders it"
	reasonTUIOnlyOff   = "declared TUI-only on its view: a one-shot surface cannot render it"
	reasonOfflineRoot  = "a local command at an offline root, run by `ze <root>` with no daemon"
	reasonOfflineCLI   = "a local-data command at an offline root: `ze cli` answers it in its own process"
	reasonDaemonPath   = "a local command at an offline root: the daemon surfaces offer its declared daemon path instead"
)

// AnswerSurface answers what surface s owes reg, and why. It is the one place
// that decides it: every surface producer (the SSH tree, the web admin tree,
// the web terminal completer, the MCP, REST and gRPC command list, the
// `ze cli` tree, shell completion) and the parity check read this answer.
//
// The answer is derived from the registration alone:
//   - a hidden command is offered nowhere;
//   - a view with no other registration is offered only on SSH and `ze cli`,
//     and only when its view registration declares it TUI-only; an undeclared
//     one is refused, because no surface could serve it as a command;
//   - a streaming command is offered streaming on the three terminal surfaces
//     (SSH, `ze cli`, shell); a one-shot surface does not offer one declared
//     streaming-only, and answers any other by its kind (its one-shot RPC
//     answers one sample);
//   - a local or local-data command at an offline root declares the YANG path
//     that answers it on the daemon (DaemonPath): shell completion offers the
//     root, `ze cli` offers a local-data one (it answers local-data in its own
//     process), and the daemon surfaces offer the declared path, judged as its
//     own registration, rather than a path no YANG node can carry;
//   - a builtin, local or local-data command is served by the daemon and by
//     `ze <verb>`, so every surface offers it;
//   - a plugin command with no YANG node exists only in a running daemon's
//     registry, so every daemon surface offers it and shell completion does not;
//   - a plugin command whose path is a schema node with no `ze:command` of its
//     own (Modeled) is offered by shell completion too, which walks the schema.
func AnswerSurface(reg CommandRegistration, s Surface) (SurfaceAnswer, error) {
	if s == SurfaceUnspecified {
		return SurfaceAnswer{}, fmt.Errorf("command %q: the surface is unspecified", reg.Path)
	}
	if s > SurfaceShell {
		return SurfaceAnswer{}, fmt.Errorf("command %q: no answer for surface %s", reg.Path, s)
	}
	if reg.Kind == KindUnspecified {
		return SurfaceAnswer{}, fmt.Errorf("command %q: registration kind is unspecified", reg.Path)
	}
	if reg.Modeled && reg.Kind != KindPlugin {
		return SurfaceAnswer{}, fmt.Errorf("command %q: kind %s is declared modeled; only a plugin command may be", reg.Path, reg.Kind)
	}
	if reg.Hidden {
		return SurfaceAnswer{Offer: OfferMustNot, Reason: reasonHidden}, nil
	}
	if reg.Kind == KindView {
		return answerView(reg, s)
	}
	if reg.StreamingOnly && !reg.Streaming {
		return SurfaceAnswer{}, fmt.Errorf("command %q: declared streaming-only with no streaming handler", reg.Path)
	}
	if reg.Streaming && s.terminal() {
		return SurfaceAnswer{Offer: OfferStreaming, Reason: reasonStreaming}, nil
	}
	if reg.StreamingOnly {
		return SurfaceAnswer{Offer: OfferMustNot, Reason: reasonStreamingOff}, nil
	}
	if reg.DaemonPath != "" {
		return answerOfflineRoot(reg, s)
	}
	switch reg.Kind {
	case KindBuiltin, KindLocal, KindLocalData:
		return SurfaceAnswer{Offer: OfferMust, Reason: reasonServed}, nil
	case KindPlugin:
		if s == SurfaceShell {
			if reg.Modeled {
				return SurfaceAnswer{Offer: OfferMust, Reason: reasonPluginSchema}, nil
			}
			return SurfaceAnswer{Offer: OfferMustNot, Reason: reasonPluginShell}, nil
		}
		return SurfaceAnswer{Offer: OfferMust, Reason: reasonPluginDaemon}, nil
	case KindUnspecified, KindView:
		// Answered above; reaching here is a defect in this function.
		return SurfaceAnswer{}, fmt.Errorf("command %q: kind %s reached the kind switch", reg.Path, reg.Kind)
	}
	return SurfaceAnswer{}, fmt.Errorf("command %q: no answer for registration kind %s", reg.Path, reg.Kind)
}

// answerOfflineRoot answers a local command whose daemon form is another path:
// listed on shell completion, which runs `ze <root>` locally, and on `ze cli`
// when it is local-data, since `ze cli` answers local-data in its own process
// (command.ServeLocal) before it asks the daemon. No daemon surface lists it:
// each offers the declared daemon path instead.
func answerOfflineRoot(reg CommandRegistration, s Surface) (SurfaceAnswer, error) {
	if reg.Kind != KindLocal && reg.Kind != KindLocalData {
		return SurfaceAnswer{}, fmt.Errorf("command %q: kind %s declares a daemon path; only a local command at an offline root may", reg.Path, reg.Kind)
	}
	if s == SurfaceShell {
		return SurfaceAnswer{Offer: OfferMust, Reason: reasonOfflineRoot}, nil
	}
	if s == SurfaceCLI && reg.Kind == KindLocalData {
		return SurfaceAnswer{Offer: OfferMust, Reason: reasonOfflineCLI}, nil
	}
	return SurfaceAnswer{Offer: OfferMustNot, Reason: reasonDaemonPath + " (" + reg.DaemonPath + ")"}, nil
}

// answerView answers a view with no other registration: TUI-only when its view
// registration declares it, refused otherwise.
func answerView(reg CommandRegistration, s Surface) (SurfaceAnswer, error) {
	if !reg.TUIOnly {
		return SurfaceAnswer{}, fmt.Errorf("command %q: a view with no daemon RPC must be declared TUI-only on its view registration (cli.RegisterView), or given an RPC", reg.Path)
	}
	if s == SurfaceSSH {
		return SurfaceAnswer{Offer: OfferTUIOnly, Reason: reasonTUIOnly}, nil
	}
	if s == SurfaceCLI {
		return SurfaceAnswer{Offer: OfferTUIOnly, Reason: reasonTUIOnly}, nil
	}
	return SurfaceAnswer{Offer: OfferMustNot, Reason: reasonTUIOnlyOff}, nil
}

// terminal answers whether s runs a command on an operator's terminal, where a
// stream can be shown live: SSH, `ze cli` and the local shell.
func (s Surface) terminal() bool {
	switch s {
	case SurfaceSSH, SurfaceCLI, SurfaceShell:
		return true
	case SurfaceUnspecified, SurfaceWebAdmin, SurfaceWebCompletion, SurfaceMCP, SurfaceREST, SurfaceGRPC:
		return false
	}
	panic("BUG: Surface.terminal: surface outside the declared set, unchecked by AnswerSurface")
}

// OffersCommand answers whether surface s lists reg: AnswerSurface's offer,
// reduced to listed or not, for a producer that builds a list.
func OffersCommand(reg CommandRegistration, s Surface) (bool, error) {
	answer, err := AnswerSurface(reg, s)
	if err != nil {
		return false, err
	}
	return answer.Offer.Listed(), nil
}

// ListedOnDaemonSurfaces answers whether every surface that reads the running
// daemon's registry (every surface but shell completion) lists reg. A producer
// that feeds one list to all of them, such as the plugin command registry,
// asks this. The surfaces answer alike for every kind today; if AnswerSurface
// ever splits them, this refuses rather than pick one, and the producer then
// needs a surface of its own.
func ListedOnDaemonSurfaces(reg CommandRegistration) (bool, error) {
	var first Surface
	var listed bool
	for _, s := range Surfaces() {
		if s == SurfaceShell {
			continue // shell completion runs with no daemon to read
		}
		got, err := OffersCommand(reg, s)
		if err != nil {
			return false, err
		}
		if first == SurfaceUnspecified {
			first, listed = s, got
			continue
		}
		if got != listed {
			return false, fmt.Errorf("command %q: offered=%v on %s but %v on %s, so one list cannot feed both", reg.Path, listed, first, got, s)
		}
	}
	return listed, nil
}
