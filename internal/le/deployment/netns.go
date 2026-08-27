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

package deployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
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
const (
	ipNetns = "netns"
	ipLink  = "link"
	ipAddr  = "addr"
	ipName  = "name"
	ipSet   = "set"
	ipAdd   = "add"
	ipShow  = "show"
	ipType  = "type"
	ipDev   = "dev"
)

// netnsDir is where `ip netns` keeps the bind mount that names a namespace. It
// is created before the first namespace is added, because ip does not create it
// and answers a bare "No such file or directory" when it is missing.
const netnsDir = "/run/netns"

// settleGrace is the time between the signal that asks namespace processes to
// leave and the signal that makes them leave.
//
// The grace is short because it applies to a daemon and a peer that have already
// answered the run's question. The grace permits a clean unwind of the kernel's
// L2TP and PPP state. The run then verifies that this state returned to its
// initial value.
const settleGrace = 200 * time.Millisecond

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
func nsCommand(ns string, argv ...string) *exec.Cmd {
	full := nsArgv(ns, argv...)
	return exec.CommandContext(context.Background(), full[0], full[1:]...) //nolint:gosec // the argv is this package's own, never an operator's
}

// hostText runs one command on the host. It answers the combined output and
// reports whether the command succeeded.
//
// It captures both streams. ip writes its diagnosis to standard error and its
// answer to standard output. A caller that reports a failure needs the stream
// that contains the reason.
func hostText(name string, argv ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), netnsTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, name, argv...).CombinedOutput() //nolint:gosec // the argv is this package's own, never an operator's
	return string(out), err == nil
}

// nsText runs one command inside ns and answers its combined output and whether
// it succeeded.
func nsText(ns string, argv ...string) (string, bool) {
	full := nsArgv(ns, argv...)
	return hostText(full[0], full[1:]...)
}

// hostRequired runs one command on the host. It answers an error that names the
// failed step and writes the command's own output to the progress stream.
//
// The error carries the step's name instead of its argv. The argv is already on
// the progress stream. The name tells an operator which part of the setup
// refused.
func hostRequired(progress io.Writer, what string, argv ...string) error {
	out, ok := hostText(argv[0], argv[1:]...)
	if ok {
		return nil
	}
	writeProgress(progress, out)
	var tb textbuf.Buffer
	return errors.New(tb.Str(what).Str(" failed").String())
}

// nsRequired runs one command inside ns on the same terms as hostRequired.
func nsRequired(progress io.Writer, ns, what string, argv ...string) error {
	return hostRequired(progress, what, nsArgv(ns, argv...)...)
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

// killNamespaceProcesses sends sig to every process in ns.
//
// A namespace that does not exist or contains no process is not an error. This
// function runs on the way in and on the way out. A first run has no namespace
// and therefore no process in it.
func killNamespaceProcesses(ns string, sig syscall.Signal) {
	out, ok := nsPids(ns)
	if !ok {
		return
	}
	for field := range strings.FieldsSeq(out) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		// A process that has already exited, or that this run lacks permission to
		// signal, gives the caller nothing it can act on. The caller's next step is
		// the namespace delete, which reports its own failure.
		syscall.Kill(pid, sig) //nolint:errcheck // see above
	}
}

// nsPids answers what `ip netns pids` said about ns.
func nsPids(ns string) (string, bool) {
	return hostText("ip", ipNetns, "pids", ns)
}

// removeNamespaces stops all processes in each namespace and deletes each
// namespace. It then deletes each link.
//
// It does not report failures. On the way in, nothing exists yet. On the way
// out, the caller reads the run's verdict. The order matters. The processes
// stop first because a namespace that contains a live process is not deleted.
func removeNamespaces(namespaces, links []string) {
	for _, ns := range namespaces {
		killNamespaceProcesses(ns, syscall.SIGTERM)
	}
	time.Sleep(settleGrace)
	for _, ns := range namespaces {
		killNamespaceProcesses(ns, syscall.SIGKILL)
	}
	for _, link := range links {
		hostText("ip", ipLink, "delete", link) //nolint:errcheck // cleanup
	}
	// The code deletes namespaces in the REVERSE of their creation order. Every
	// unwind removes the last item built first. This order handles first the
	// namespace most likely to refuse because it still holds the far end of a
	// link that has just been deleted.
	for _, ns := range slices.Backward(namespaces) {
		hostText("ip", ipNetns, "delete", ns) //nolint:errcheck // cleanup
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
