// Design: docs/architecture/testing/interop.md -- ze against a real peer
// Overview: deployment.go -- the collector and the process handles these commands feed
// Related: l2tpppp.go -- the proof that builds two namespaces and a veth pair
// Related: gokrazylab.go -- the appliance lab built out of these primitives
// Related: hostkernel.go -- the gate that runs before any namespace is made
// Related: pppstate.go -- the questions put inside these namespaces
//
// netns.go provides the Linux network namespace machinery that the on-host
// proofs share. A proof that runs ze and somebody else's daemon on ONE machine
// must keep them apart. Both want to bind the same L2TP port and own a PPP
// interface. A namespace pair joined by a veth gives each daemon its own stack.
// Every command either daemon runs is prefixed with its namespace.
//
// Every command here resolves through PATH. deployment.go states the reason.
// The proof IS the argv that reaches ip. A seam that replaces it tests a
// different program.

package testdeployment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// netnsTimeout bounds one namespace command. A link change, an address change,
// or a ping with its own -W bound communicates with the kernel and answers in
// milliseconds. Thus, thirty seconds means the netlink socket is not answering.
const netnsTimeout = 30 * time.Second

// These are the ip sub-command words that this package uses more than once.
// They are part of ip's own grammar, not one proof's grammar. Their names show a
// reader that every use has the same word.
// enumeration: exempt (these are ip(8) subcommand words, named here because this package uses each more than once, not the CLI verb vocabulary)
const (
	ipNetns = "netns"
	ipLink  = "link"
	ipAddr  = "addr"
	ipName  = "name"
	ipSet   = "set"
	ipAdd   = "add"
	ipShow  = "show"
	ipL2TP  = "l2tp"
	ipType  = "type"
	ipDev   = "dev"
	ipPeer  = "peer"
)

// These are the words of ze's own environment and of le's command tree that
// more than one proof sets or runs.
const (
	// zeLogDebug and zeLogInfo are the ze log levels a proof's daemon runs at.
	zeLogDebug = "debug"
	zeLogInfo  = "info"
	// zeConfigDirVariable names the directory a containerized ze reads.
	zeConfigDirVariable = "ZE_CONFIG_DIR"
	// leTestArea is le's test area, under which the scale and peer harnesses run.
	leTestArea = "test"
)

// netnsDir is where `ip netns` keeps the bind mount that names a namespace. It
// is created before the first namespace is added, because ip does not create it
// and answers a bare "No such file or directory" when it is missing.
const netnsDir = "/run/netns"

// nsArgv answers argv, run inside the namespace ns.
func nsArgv(ns string, argv ...string) []string {
	full := make([]string, 0, len(argv)+4)
	full = append(full, "ip", ipNetns, "exec", ns)
	return append(full, argv...)
}

// nsCommand answers the command that runs argv inside ns, unbounded in time.
//
// The command is unbounded because its two callers are a daemon and a peer.
// They run until the proof stops them. The waits in the run set the bounds. A
// deadline here would stop a working daemon in the middle of a session.
func nsCommand(ctx context.Context, ns string, argv ...string) *exec.Cmd {
	full := nsArgv(ns, argv...)
	return exec.CommandContext(ctx, full[0], full[1:]...) //nolint:gosec // The namespace and argv belong to the native proof.
}

// hostText runs one command on the host. It answers the combined output and
// reports whether the command succeeded.
//
// It captures both streams. ip writes its diagnosis to standard error and its
// answer to standard output. A caller that reports a failure needs the stream
// that contains the reason.
func hostText(ctx context.Context, name string, argv ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, netnsTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, name, argv...) //nolint:gosec // The argv belongs to the native proof.
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	err := runNativeCommand(ctx, command)
	return output.String(), err == nil
}

// nsText runs one command inside ns and answers its combined output and whether
// it succeeded.
func nsText(ctx context.Context, ns string, argv ...string) (string, bool) {
	full := nsArgv(ns, argv...)
	return hostText(ctx, full[0], full[1:]...)
}

// hostRequired runs one command on the host. It answers an error that names the
// failed step and writes the command's own output to the progress stream.
//
// The error carries the step's name instead of its argv. The argv is already on
// the progress stream. The name tells an operator which part of the setup
// refused.
func hostRequired(ctx context.Context, progress io.Writer, what string, argv ...string) error {
	out, ok := hostText(ctx, argv[0], argv[1:]...)
	if ok {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	writeProgress(progress, out)
	var tb textbuf.Buffer
	return errors.New(tb.Str(what).Str(" failed").String())
}

// nsRequired runs one command inside ns on the same terms as hostRequired.
func nsRequired(ctx context.Context, progress io.Writer, ns, what string, argv ...string) error {
	return hostRequired(ctx, progress, what, nsArgv(ns, argv...)...)
}

// writeProgress puts text on the progress stream, adding the newline it lacks.
// A nil stream discards it, which is what a test that wants only the report
// passes.
func writeProgress(progress io.Writer, text string) {
	if progress == nil || text == "" {
		return
	}
	var tb textbuf.Buffer
	tb.Str(text)
	if !strings.HasSuffix(text, "\n") {
		tb.Byte('\n')
	}
	io.WriteString(progress, tb.String()) //nolint:errcheck // progress output
}

// ensureNetnsDir creates the directory `ip netns` keeps its namespaces in.
func ensureNetnsDir() error { return os.MkdirAll(netnsDir, 0o750) }

// removeNamespaces deletes only the run's named links and namespaces. The
// caller MUST first join every process owner; namespace PID listings are not
// authority to signal another process.
func removeNamespaces(ctx context.Context, namespaces, links []string) {
	for _, link := range links {
		hostText(ctx, "ip", ipLink, "delete", link) //nolint:errcheck // Cleanup may find an object already absent.
	}
	// The code deletes namespaces in the REVERSE of their creation order. Every
	// unwind removes the last item built first. This order handles first the
	// namespace most likely to refuse because it still holds the far end of a
	// link that has just been deleted.
	for _, ns := range slices.Backward(namespaces) {
		hostText(ctx, "ip", ipNetns, "delete", ns) //nolint:errcheck // Cleanup may find a namespace already absent.
	}
}

// namespaceSuffix answers the process id every namespace and link this run
// makes is named after, so two runs on one machine do not collide.
func namespaceSuffix() string {
	var tb textbuf.Buffer
	return tb.Int(int64(os.Getpid())).String()
}

// linkSuffix answers the last digits of the namespace suffix.
//
// The kernel limits a link name to 15 characters. A process id does not fit in
// that limit with a readable prefix. Thus, the low digits name the link. Those
// digits differ between two runs on one machine.
func linkSuffix(suffix string) string {
	const keep = 6
	if len(suffix) <= keep {
		return suffix
	}
	return suffix[len(suffix)-keep:]
}
