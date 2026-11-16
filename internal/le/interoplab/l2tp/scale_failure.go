// Design: docs/labs/l2tp-interop.md -- bounded peer evidence at a failed assertion.
// Related: scale.go -- captures this evidence before Suite cleanup removes peers.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const scalePeerLogPath = "/run/kl2tpd.log"

// startScalePeer retains the detached daemon's own output, which Docker's
// container log cannot see. The caller MUST measure and release this tunnel's
// window before starting another peer. Suite cleanup MUST stop its children.
func startScalePeer(ctx context.Context, lab labOperations, lac int) error {
	return lab.ExecDetached(ctx, "kl2tpd-"+strconv.Itoa(lac), []string{
		"sh", "-c", `exec kl2tpd -config "$1" >"$2" 2>&1`,
		"scale-peer", scaleConfigPath, scalePeerLogPath,
	}, nil)
}

// scaleFailureDiagnostics MUST run before checkScaleSessions returns its failed
// assertion to Suite cleanup. checkScaleSessions MUST preserve the assertion
// cause. Twelve reads share one independent deadline, even after cancellation.
func scaleFailureDiagnostics(ctx context.Context, lab labOperations, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	failures := make([]error, 0, 1+4*scaleLACCount)
	failures = append(failures, cause)
	var diagnostics textbuf.Buffer
	for lac := 1; lac <= scaleLACCount; lac++ {
		peer := "kl2tpd-" + strconv.Itoa(lac)
		for _, command := range scaleFailureCommands(lac) {
			result, err := lab.Exec(ctx, peer, command, nil)
			diagnostics.Str("\n--- L2TP-SCALE-PEER-FAILURE ").Str(peer).Byte(' ').Str(strings.Join(command, " ")).Str(" ---\n")
			output := result.Stdout + result.Stderr
			if output == "" {
				diagnostics.Str("(no output)\n")
			} else {
				diagnostics.Str(scaleFailureExcerpt(output)).Byte('\n')
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("scale peer %s diagnostic %s: %w", peer, command[0], err))
			}
		}
	}
	return fmt.Errorf("%w%s", errors.Join(failures...), diagnostics.String())
}

// scaleFailureCommands reads each peer through four bounded commands. Each PPP
// or PID read takes all sixteen explicit filenames in one process, not one
// process per file. Missing files remain diagnostic errors and partial output.
func scaleFailureCommands(lac int) [][]string {
	ppp := make([]string, 3, 3+scaleSessionsPerLAC)
	copy(ppp, []string{"tail", "-c", "4096"})
	pids := make([]string, 3, 3+scaleSessionsPerLAC)
	copy(pids, []string{"head", "-c", "256"})
	for number := 1; number <= scaleSessionsPerLAC; number++ {
		user := scaleUsername(lac, number)
		ppp = append(ppp, "/run/"+user+".log")
		pids = append(pids, "/var/run/pppd/ppp-"+user+".pid")
	}
	return [][]string{
		{"tail", "-c", "16384", scalePeerLogPath},
		ppp,
		pids,
		{"sh", "-c", "ps | head -c 16384"},
	}
}

// scaleFailureExcerpt bounds even an unexpectedly oversized command result;
// 80 KiB preserves sixteen 4-KiB file tails and their filenames without clipping.
func scaleFailureExcerpt(output string) string {
	const limit = 80 * 1024
	if len(output) <= limit {
		return output
	}
	return output[:limit] + "\n[diagnostic output truncated]"
}
