// Design: docs/architecture/resolve.md -- RIR delegation lookup CLI
package cli

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdRIR answers which Regional Internet Registry holds an AS number. It
// reaches no network and no daemon: the delegation table ships with the
// binary, so the answer is available on a laptop with the cable unplugged.
//
// A write to the terminal that fails has nowhere left to report itself, and
// the exit code already carries the failure, so each write below drops its
// error.
func cmdRIR(args []string) int {
	if len(args) > 0 && isHelp(args[0]) {
		rirUsage()
		return exitOK
	}

	if len(args) != 1 {
		rirUsage()
		return exitError
	}

	// ParseUint with a bit size of 32 bounds the AS number to uint32, so the
	// conversion below cannot truncate.
	asn, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		var tb textbuf.Buffer
		_ = tb.Str("error: invalid AS number: ").Str(args[0]).Byte('\n').StdErr()
		return exitError
	}

	entry, err := irr.RegistryForASN(uint32(asn))
	if errors.Is(err, irr.ErrASNUnallocated) {
		var tb textbuf.Buffer
		_ = tb.Str("AS").Uint(asn).Str(" is in no delegated range\n").StdErr()
		return exitError
	}
	if err != nil {
		var tb textbuf.Buffer
		_ = tb.Str("error: ").Err(err).Byte('\n').StdErr()
		return exitError
	}

	var tb textbuf.Buffer
	_ = tb.Str(entry.RIR).Byte(' ').Str(entry.Whois).Byte('\n').StdOut()

	return exitOK
}

func rirUsage() {
	p := helpfmt.Page{
		Command: "ze resolve rir",
		Summary: "which Regional Internet Registry holds an AS number",
		Usage:   []string{"ze resolve rir <asn>"},
		Sections: []helpfmt.HelpSection{
			{Title: usageSectionOperations, Entries: []helpfmt.HelpEntry{
				{Name: "<asn>", Desc: "AS number to look up in the RIR delegation table"},
			}},
		},
		Examples: []string{
			"ze resolve rir 15169",
			"ze resolve rir 4294967295",
		},
	}
	p.WriteErr()
}
