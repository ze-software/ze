// Design: docs/architecture/diagnostics/path-mtu.md -- the show mtu handler and its payload
// Related: register.go -- the RPC registration that reaches handleShowMTU

package cmd

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
)

// envKeyReferenceAddress overrides the environment mtu reference-address leaf
// of ze-mtu-conf.yang. The config package plumbs the leaf into this key
// (envPlumbingTable, internal/component/config/apply_env.go), so one read here
// answers with the OS environment first, the config value second and the YANG
// default last.
const envKeyReferenceAddress = "ze.mtu.reference-address"

// referenceAddressEntry registers the key. Its Default repeats the YANG
// default of the leaf, as every plumbed environment leaf's registration does
// (internal/component/config/environment.go), because env.Get answers the
// empty string when neither the OS environment nor the config names a value,
// and referenceAddress reads the default from this entry rather than from a
// third declaration.
//
//nolint:gochecknoglobals // Registration handle, read once per run.
var referenceAddressEntry = env.MustRegister(env.EnvEntry{
	Key:         envKeyReferenceAddress,
	Type:        "string",
	Default:     "1.1.1.1",
	Description: "Address show mtu measures beside the peers, to tell a clamped circuit from a clamped peer path",
})

// The keywords of `show mtu [host <address>] [exhaustive] [detail]`. The two bare
// words are the enumeration values of the search and view leaves in
// ze-mtu-cmd.yang, which is what lets the grammar offer them.
const (
	argHost       = "host"
	argExhaustive = "exhaustive"
	argDetail     = "detail"
)

// The keys of the answer. Every key is kebab-case and every later phase fills
// these rather than reshaping them (docs/architecture/diagnostics/path-mtu.md,
// "The payload").
const (
	fieldStatus       = "status"
	fieldVerdict      = "verdict"
	fieldMeasurements = "measurements"
	fieldReference    = "reference"
	fieldUnderlay     = "underlay"
	fieldTunnels      = "tunnels"
	fieldCommands     = "commands"
	fieldNotes        = "notes"
	fieldCaveats      = "caveats"
	fieldSeverity     = "severity"
	fieldText         = "text"
)

// runStatus is the outcome of one run, the same four outcomes the ported
// tool's exit codes carried. The zero value is Unspecified so an unset field
// can never pass for an answer.
type runStatus uint8

const (
	runStatusUnspecified runStatus = iota
	// runStatusOK: at least one path was measured and the DF gate held.
	runStatusOK
	// runStatusNothingMeasured: no target answered a single probe.
	runStatusNothingMeasured
	// runStatusDFGateFailed: a probe the path must refuse was answered, so
	// Don't Fragment is not honored and no figure can be believed.
	runStatusDFGateFailed
)

// String answers the wire spelling of the status. It is written into the
// payload and never compared.
func (s runStatus) String() string {
	switch s {
	case runStatusOK:
		return "ok"
	case runStatusNothingMeasured:
		return "nothing-measured"
	case runStatusDFGateFailed:
		return "df-gate-failed"
	default:
		panic("BUG: runStatus written to the payload before it was set")
	}
}

// runVerdict is the run-level ladder runVerdictOf (verdict.go) applies, first
// match wins: action-needed when any tunnel is oversized or has no usable
// value, or any fault outside the tunnel table was found (a refused
// transform is one); check when any tunnel is tight or down; no-tunnels when
// no peer exists; ok otherwise, an under-utilized tunnel included. The ported
// tool's DO NOT APPLY rung has no counterpart, because the overhead is
// derived per negotiated transform rather than assumed. The zero value is
// Unspecified so an unset field can never pass for an answer.
type runVerdict uint8

const (
	runVerdictUnspecified runVerdict = iota
	runVerdictOK
	runVerdictCheck
	runVerdictActionNeeded
	runVerdictNoTunnels
)

// String answers the wire spelling of the verdict. It is written into the
// payload and never compared.
func (v runVerdict) String() string {
	switch v {
	case runVerdictOK:
		return "ok"
	case runVerdictCheck:
		return "check"
	case runVerdictActionNeeded:
		return "action-needed"
	case runVerdictNoTunnels:
		return "no-tunnels"
	default:
		panic("BUG: runVerdict written to the payload before it was set")
	}
}

// noteSeverity is the weight of one note. The zero value is Unspecified so an
// unset field can never pass for an answer.
type noteSeverity uint8

const (
	noteSeverityUnspecified noteSeverity = iota
	noteSeverityInfo
	noteSeverityCaution
	noteSeverityFault
)

// String answers the wire spelling of the severity. It is written into the
// payload and never compared.
func (s noteSeverity) String() string {
	switch s {
	case noteSeverityInfo:
		return "info"
	case noteSeverityCaution:
		return "caution"
	case noteSeverityFault:
		return "fault"
	default:
		panic("BUG: noteSeverity written to the payload before it was set")
	}
}

// mtuRequest is one parsed `show mtu` invocation. host is the one address to
// measure, and the zero Addr means the peers and the reference address are
// measured instead. exhaustive discards the kernel's cached path MTU and probes
// every size on the wire. detail adds
// every probe sent and every reply received to each measurement.
type mtuRequest struct {
	host       netip.Addr
	exhaustive bool
	detail     bool
}

// handleShowMTU is the RPC handler for `show mtu` (ze-show:mtu). It parses
// the request, validates the configured reference address, and answers the
// payload of one run (run.go, runMTU) over the live dependencies. The
// command is read-only: no path from here reaches a configuration write.
func handleShowMTU(cctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	req, err := parseMTUArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	if _, refErr := referenceAddress(); refErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: refErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(runMTU(requestContext(cctx), &req, &liveDeps))}, nil
}

// requestContext is the caller's context when the transport supplied one.
// The dispatcher's tests and the wiring tests drive the handler with no
// CommandContext, and a run without a caller is not cancellable.
func requestContext(cctx *pluginserver.CommandContext) context.Context {
	if cctx == nil {
		return context.Background()
	}
	if cctx.RequestContext == nil {
		return context.Background()
	}
	return cctx.RequestContext
}

// parseMTUArgs parses `show mtu [host <address>] [exhaustive] [detail]`. The
// dispatcher has already refused a host value the ip-address type does not
// admit, so the parse here is the second check of the pair. A token that is
// none of the three keywords is refused by name, and so is a dash-leading
// value, which is an option and never data (ai/rules/cli.md).
func parseMTUArgs(args []string) (mtuRequest, error) {
	var req mtuRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argHost:
			if req.host.IsValid() {
				return mtuRequest{}, fmt.Errorf("mtu: duplicate keyword %q", argHost)
			}
			if i+1 >= len(args) {
				return mtuRequest{}, fmt.Errorf("mtu: %s requires an IPv4 or IPv6 address", argHost)
			}
			i++
			if strings.HasPrefix(args[i], "-") {
				return mtuRequest{}, fmt.Errorf("mtu: %q is an option, and %s takes an address", args[i], argHost)
			}
			addr, err := netip.ParseAddr(args[i])
			if err != nil {
				return mtuRequest{}, fmt.Errorf("mtu: invalid host %q: %w", args[i], err)
			}
			req.host = addr
		case argExhaustive:
			if req.exhaustive {
				return mtuRequest{}, fmt.Errorf("mtu: duplicate keyword %q", argExhaustive)
			}
			req.exhaustive = true
		case argDetail:
			if req.detail {
				return mtuRequest{}, fmt.Errorf("mtu: duplicate keyword %q", argDetail)
			}
			req.detail = true
		default:
			return mtuRequest{}, fmt.Errorf("mtu: unknown argument %q; the keywords are %s <address>, %s and %s", args[i], argHost, argExhaustive, argDetail)
		}
	}
	return req, nil
}

// referenceAddress answers the configured reference address, read through
// the env layer so the OS environment, the config leaf and the default apply
// in that order. The empty string is what env.Get answers when nothing set
// the key, and it means the default, not no reference. A value that is not
// an address is refused by name rather than read as no reference, because no
// reference is a different outcome (the underlay advice becomes undecidable)
// and the two must not be confused.
func referenceAddress() (netip.Addr, error) {
	value := env.Get(envKeyReferenceAddress)
	if value == "" {
		value = referenceAddressEntry.Default
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("mtu: %s is %q, which is not an IPv4 or IPv6 address: %w", envKeyReferenceAddress, value, err)
	}
	return addr, nil
}
