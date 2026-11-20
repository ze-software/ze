// Design: docs/architecture/api/process-protocol.md -- owned interface devices and firewall table contribution
// Related: register.go -- the grant, the reservation and the registration
// Related: namespace_linux.go -- the network namespace the link request targets
//
// Package fakeowner is a test-only plugin that calls the interface and
// firewall engine operations the way a feature plugin would: it declares one
// owned bridge, contributes one reserved firewall table, and asks for one link
// into a network namespace it holds. It records each answer and serves them as
// `show fakeowner outcome`, the barrier a .ci waits on before it reads the
// kernel. It runs internal (direct path) and forked (RPC) alike.
package fakeowner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// outcomeCommand is the one command the plugin serves.
const outcomeCommand = "show fakeowner outcome"

// requestTimeout bounds each engine request.
const requestTimeout = 10 * time.Second

// The fixed values of the requests, as the .ci files read them back.
const (
	bridgeName = devicePrefix + "-wire"
	bridgeMTU  = 1500
	linkKey    = "wire"
	linkName   = "eth0"
	linkMAC    = "02:00:5e:00:53:02"
)

// bridgeAddress is the bridge's gateway address, and linkAddress the peer
// end's address in the same prefix; the peer's default route goes through
// the gateway.
var (
	bridgeAddress = netip.MustParsePrefix("192.0.2.1/24")
	linkAddress   = netip.MustParsePrefix("192.0.2.2/24")
)

// tableSection is the contributed table, in the firewall config section form:
// one filter base chain at the input hook that accepts.
const tableSection = `{"firewall":{"table":{"` + tableName + `":{"family":"inet","chain":{"input":{"type":"filter","hook":"input","priority":"0","policy":"accept"}}}}}}`

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(logger *slog.Logger) { loggerPtr.Store(logger) }

func logger() *slog.Logger {
	if logger := loggerPtr.Load(); logger != nil {
		return logger
	}
	return slog.Default()
}

// outcomes holds the answer of each request, as a token a .ci needle names
// ("bridge-accepted") followed by the reason of a refusal. Safe for
// concurrent use: the command handler reads while the requests write.
var outcomes = struct {
	mu     sync.Mutex
	values map[string]string
}{
	values: map[string]string{"bridge": "bridge-pending", "link": "link-pending", "table": "table-pending"},
}

func recordOutcome(subject string, err error) {
	value := subject + "-accepted"
	if err != nil {
		value = subject + "-refused: " + strings.ReplaceAll(err.Error(), "\n", " / ")
	}
	outcomes.mu.Lock()
	outcomes.values[subject] = value
	outcomes.mu.Unlock()
}

// runPlugin is the engine-mode entry point, internal and forked.
func runPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	// The holder lives until the plugin ends, so the link's namespace does too.
	// The started callback stores it and this function's defer reads it, on
	// two goroutines, hence the atomic.
	var holder atomic.Pointer[namespaceHolder]
	defer func() {
		if held := holder.Load(); held != nil {
			held.close()
		}
	}()

	p.OnExecuteCommand(dispatchCommand)
	p.OnStarted(func(startedCtx context.Context) error {
		holder.Store(makeRequests(ctx, startedCtx, p))
		return nil
	})

	if err := p.Run(ctx, &sdk.Registration{Commands: commandDecls()}); err != nil {
		logger().Error(Name+" plugin failed", "error", err, "code", "doctor-plugin-crashed")
		return 1
	}
	return 0
}

// makeRequests asks for the bridge, the table and the link, in that order
// because the link joins the bridge, and records each answer. runCtx owns the
// namespace holder's process; startedCtx bounds the requests.
func makeRequests(runCtx, startedCtx context.Context, p *sdk.Plugin) *namespaceHolder {
	requestCtx, cancel := context.WithTimeout(startedCtx, requestTimeout)
	defer cancel()

	_, err := p.OwnedBridgesReplace(requestCtx, &rpc.OwnedBridgesReplaceInput{
		Bridges: []rpc.OwnedBridge{{Name: bridgeName, MTU: bridgeMTU, Addresses: []netip.Prefix{bridgeAddress}}},
	})
	recordOutcome("bridge", err)

	_, err = p.FirewallTablesReplace(requestCtx, &rpc.FirewallTablesReplaceInput{Section: json.RawMessage(tableSection)})
	recordOutcome("table", err)

	holder, err := startNamespaceHolder(runCtx)
	if err != nil {
		recordOutcome("link", fmt.Errorf("no namespace to target: %w", err))
		return nil
	}
	_, err = p.OwnedLinkCreate(requestCtx, &rpc.OwnedLinkCreateInput{
		Key:             linkKey,
		Bridge:          bridgeName,
		Slot:            1,
		PID:             holder.pid,
		NamespaceInode:  holder.inode,
		Name:            linkName,
		HardwareAddress: linkMAC,
		MTU:             bridgeMTU,
		Addresses:       []netip.Prefix{linkAddress},
		Gateways:        []netip.Addr{bridgeAddress.Addr()},
	})
	recordOutcome("link", err)
	if err == nil {
		recordHolder(holder.pid)
	}
	return holder
}

// recordHolder serves the namespace holder's process id under link-pid, so a
// .ci reads the peer end inside the namespace it names
// (`le test fixture iface/peer-end`).
func recordHolder(pid int) {
	outcomes.mu.Lock()
	outcomes.values["link-pid"] = strconv.Itoa(pid)
	outcomes.mu.Unlock()
}

// dispatchCommand serves the outcome. The answer is an object, because the
// engine refuses a bare JSON string.
func dispatchCommand(_, command string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
	if command != outcomeCommand {
		return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
	}
	outcomes.mu.Lock()
	answer := make(map[string]string, len(outcomes.values))
	maps.Copy(answer, outcomes.values)
	outcomes.mu.Unlock()
	return rpc.StatusDone, answer, nil
}

// commandDecls names the command this plugin serves. init() puts it on the
// registry.Registration and the runner sends it at Stage 1, so it MUST stay
// one function.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{{Name: outcomeCommand}}
}
