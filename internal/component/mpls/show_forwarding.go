// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` CLI (AC-7)
// Related: register.go -- registers the RPC and this package's kernel reader
// Related: forwarding_linux.go -- kernel reader (AF_MPLS swap/pop + IP-encap push)
// Related: ../../core/mplsfib/forwarding.go -- the reader registry keyed by data plane
//
// `show mpls forwarding` lists the MPLS label-switching entries installed in
// the data plane the accepted config selects at `interface { backend }`: the
// swap and pop entries keyed by incoming label, AND the label-imposition
// (push) routes, ordinary IP routes carrying a label stack and keyed by FEC
// prefix (BGP labeled-unicast or an LDP/RSVP-TE ingress). It reads the data
// plane itself rather than a daemon's in-memory view, so it reports the state
// the packets meet, as `show route` does for IP.
//
// Each data plane's reader is registered by the code that owns that data
// plane (mplsfib.RegisterForwardingReader): this package reads the kernel,
// fib-vpp reads VPP MPLS table 0. The handler looks the reader up by the
// data plane's name and names no backend itself.
package mpls

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const defaultForwardingLimit = 100_000

// forwardingReaderFor answers the reader of the data plane the accepted
// config selects. It is a test seam: the handler's contract is tested over a
// stand-in reader, because the real ones answer from the host's kernel or
// from a running VPP.
var forwardingReaderFor = selectedForwardingReader

func handleShowMPLSForwarding(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	const usage = "usage: show mpls forwarding [limit N]"
	var tb textbuf.Buffer
	limit := defaultForwardingLimit
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "limit":
			if i+1 >= len(args) {
				return &plugin.Response{Status: plugin.StatusError, Error: "limit requires a value"}, nil
			}
			n, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || n <= 0 {
				msg := tb.Str("invalid limit ").Quoted(args[i+1]).Str(": must be a positive integer").String()
				return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil //nolint:nilerr // operational error via Response
			}
			limit = n
			i++
		default:
			msg := tb.Reset().Str("unexpected argument ").Quoted(args[i]).Str("; ").Str(usage).String()
			return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
		}
	}

	reader, refusal := forwardingReaderFor(ctx)
	if refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	entries, err := reader(limit + 1)
	if err != nil {
		if operatorErr, isOperator := errors.AsType[*opererr.Error](err); isOperator {
			return plugin.OperatorErrorResponse(operatorErr), nil
		}
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error via Response
	}
	truncated := false
	if len(entries) > limit {
		entries = entries[:limit]
		truncated = true
	}
	// entries is the response's list contract: consumers (and the .ci test)
	// require it to marshal as a JSON array even on a data plane with no MPLS
	// routes. A nil slice marshals to `null`, so pin it to an empty array. A
	// reader may answer nil for an empty table; one that cannot read answers
	// an error, which never reaches this line.
	if entries == nil {
		entries = []mplsfib.ForwardingEntry{}
	}
	data := map[string]any{"entries": entries}
	if truncated {
		data["truncated"] = true
		data["limit"] = limit
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

// selectedForwardingReader answers the reader of the data plane the accepted
// config selects, or the refusal that says why none can answer.
func selectedForwardingReader(ctx *pluginserver.CommandContext) (mplsfib.ForwardingReader, *opererr.Error) {
	dataPlane, refusal := configuredDataPlane(ctx)
	if refusal != nil {
		return nil, refusal
	}
	return readerForDataPlane(dataPlane)
}

// configuredDataPlane answers the data plane `interface { backend }` selects
// in the daemon's accepted configuration, read the way reload reads it
// (pluginserver.ConfigDataPlane). A configuration that cannot be read is a
// refusal, never the kernel: answering the kernel's table on a VPP box is the
// empty table this command exists not to give.
func configuredDataPlane(ctx *pluginserver.CommandContext) (string, *opererr.Error) {
	if ctx == nil {
		return "", configurationUnavailable()
	}
	reactor := ctx.Reactor()
	if reactor == nil {
		return "", configurationUnavailable()
	}
	tree := reactor.GetConfigTree()
	if tree == nil {
		return "", configurationUnavailable()
	}
	dataPlane := pluginserver.ConfigDataPlane(tree)
	if dataPlane == "" {
		return "", opererr.New("the MPLS forwarding table cannot be read: the configuration selects no data plane",
			"this host has no default interface backend and the config names none at 'interface { backend }', "+
				"so there is no table to read",
			"set 'interface { backend <name>; }' to the data plane that forwards, then run 'ze show mpls forwarding'")
	}
	return dataPlane, nil
}

// readerForDataPlane answers the reader registered for dataPlane, or the
// refusal naming the data plane when this build holds none.
func readerForDataPlane(dataPlane string) (mplsfib.ForwardingReader, *opererr.Error) {
	reader, ok := mplsfib.ForwardingReaderFor(dataPlane)
	if !ok {
		var problem textbuf.Buffer
		problem.Str("the MPLS forwarding table cannot be read: no reader serves data plane ").Quoted(dataPlane)
		return nil, opererr.New(problem.String(),
			"the code that owns that data plane is not in this build, or does not read its label table, "+
				"so the labels it holds cannot be shown",
			"run a ze build that includes the "+dataPlane+" data plane, or select another at 'interface { backend }'")
	}
	return reader, nil
}

func configurationUnavailable() *opererr.Error {
	return opererr.New("the MPLS forwarding table cannot be read: the accepted daemon configuration is unavailable",
		"which data plane forwards (the kernel or VPP) is read from 'interface { backend }', "+
			"and an unreadable configuration names neither",
		"retry 'ze show mpls forwarding' once the daemon has finished loading its configuration")
}
