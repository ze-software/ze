// Design: docs/features/ai-first.md — what an agent session is told about this Ze
// Related: agent.go — the other agent setup commands and the four command paths
// Related: register.go — the registrations of both routes and of the drawing
//
// `show agent context` is what a session-start hook runs, so it answers a few
// plain lines an agent reads and exits 0 whatever it finds: a hook must never
// fail a session. The shell route says whether the daemon answered a dial and
// where the config lives; the daemon command says only that the caller reached
// the daemon, because a read-only caller has no other way to learn the
// daemon's config directory.

package skills

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/version"
)

// keyAgentContext is the envelope key of both routes' answer, drawn by
// drawAgentContext.
const keyAgentContext = "agent-context"

// dialBound bounds the shell route's dial of the daemon. The hook runs at the
// start of every agent session, so a machine with no daemon, or a firewall
// that drops the packet, must not hold the session for longer than this.
const dialBound = time.Second

// The guide both routes point the agent at.
const (
	guideSkill   = "ze"
	guideCommand = "ze show skill ze"
)

// daemonReached is the daemon command's daemon line: the answer itself came
// from the daemon, so there is nothing to dial.
const daemonReached = "reached: this answer came from the daemon"

// contextFacts are what both routes of show agent context say.
type contextFacts struct {
	Version      string `json:"version"`
	Skill        string `json:"skill"`
	GuideCommand string `json:"guide-command"`
}

// localContext is the shell route's answer: the shared facts, the dial's
// outcome and the config directory of the caller's own process.
type localContext struct {
	contextFacts
	Daemon          string `json:"daemon"`
	ConfigDirectory string `json:"config-directory"`
}

// localContextAnswer is localContext under its envelope key.
type localContextAnswer struct {
	Context localContext `json:"agent-context"`
}

// daemonContext is the daemon command's answer: the shared facts and that the
// caller reached the daemon. It carries no path and nothing the daemon's
// environment holds.
type daemonContext struct {
	contextFacts
	Daemon string `json:"daemon"`
}

// contextRoutes holds what the shell route asks the outside world: where the
// daemon listens, and a dial of that address. The daemon command holds the
// same value and uses neither, which its test proves. Safe for concurrent use
// when both functions are.
type contextRoutes struct {
	address func() (host, port string, err error)
	dial    func(ctx context.Context, network, address string) (net.Conn, error)
}

// systemContextRoutes resolves the address the way `ze cli` does
// (sshclient.DaemonAddress, which reads no credential) and dials over TCP.
func systemContextRoutes() contextRoutes {
	var dialer net.Dialer
	return contextRoutes{address: sshclient.DaemonAddress, dial: dialer.DialContext}
}

// showAgentContextLocal is the shell route of show agent context.
func showAgentContextLocal(_ command.ValidatedArgs) int {
	return systemContextRoutes().printLocal(context.Background())
}

// handleShowAgentContext is the daemon command of show agent context.
func handleShowAgentContext(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return systemContextRoutes().respond(), nil
}

// printLocal prints the shell route's answer through the registered drawing.
func (r contextRoutes) printLocal(ctx context.Context) int {
	answer := localContextAnswer{Context: localContext{
		contextFacts:    sharedContextFacts(),
		Daemon:          r.daemonState(ctx),
		ConfigDirectory: configDirectory(),
	}}
	return command.PrintLocalAnswer(pathShowAgentContext, command.LocalAnswer{Payload: answer})
}

// respond answers the daemon command. It reads no file and no environment,
// and dials nothing: the caller is talking to the daemon already.
func (contextRoutes) respond() *plugin.Response {
	answer := daemonContext{contextFacts: sharedContextFacts(), Daemon: daemonReached}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{keyAgentContext: answer}}
}

// sharedContextFacts is the one producer of what both routes say.
func sharedContextFacts() contextFacts {
	return contextFacts{Version: version.Short(), Skill: guideSkill, GuideCommand: guideCommand}
}

// daemonState dials the daemon's address once, bounded by dialBound, and
// closes the connection at once. An address that cannot be resolved is
// answered as the daemon not answering, with the reason: the command still
// succeeds, because a hook must never fail a session.
func (r contextRoutes) daemonState(ctx context.Context) string {
	var state textbuf.Buffer
	host, port, err := r.address()
	if err != nil {
		return state.Str("not answering: its address could not be read: ").Err(err).String()
	}
	address := net.JoinHostPort(host, port)

	dialContext, cancel := context.WithTimeout(ctx, dialBound)
	defer cancel()

	conn, err := r.dial(dialContext, "tcp", address)
	if err != nil {
		return state.Str("not answering at ").Str(address).Str(" (").Err(err).Byte(')').String()
	}
	conn.Close() //nolint:errcheck // the dial was the whole question; nothing was sent
	return state.Str("answering at ").Str(address).String()
}

// configDirectory is the config directory of the caller's own process
// (resolve.StoreDir), made absolute so the agent can use it from anywhere.
func configDirectory() string {
	directory := resolve.StoreDir("")
	if directory == "" {
		return "unknown: ze.config.dir is not set and ze could not locate its own binary to derive it"
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		var relative textbuf.Buffer
		return relative.Str(directory).Str(" (relative: the working directory could not be read: ").Err(err).Byte(')').String()
	}
	return absolute
}

// drawnContext is either route's answer as the drawing reads it.
// ConfigDirectory is nil on the daemon command's answer, which withholds it.
type drawnContext struct {
	Version         string  `json:"version"`
	Daemon          string  `json:"daemon"`
	ConfigDirectory *string `json:"config-directory"`
	Skill           string  `json:"skill"`
	GuideCommand    string  `json:"guide-command"`
}

// drawAgentContext draws either route's answer as the lines an agent reads:
// the version, the daemon, the config directory where the route gives it,
// and the guide.
func drawAgentContext(raw json.RawMessage) (string, error) {
	var answer drawnContext
	if err := json.Unmarshal(raw, &answer); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	b.Str(answer.Version).Byte('\n')
	b.Str("daemon: ").Str(answer.Daemon).Byte('\n')
	if answer.ConfigDirectory != nil {
		b.Str("config directory: ").Str(*answer.ConfigDirectory).Byte('\n')
	}
	b.Str("guide: the ").Str(answer.Skill).Str(" skill, read it with ").Str(answer.GuideCommand).Byte('\n')
	return b.String(), nil
}
