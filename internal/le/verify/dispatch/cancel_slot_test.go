// Design: docs/contributing/running-commands.md -- canceling an owned admission wait.
// Related: cancel_group_test.go -- the real registered stage dispatch fixture.

package verifydispatch

import (
	"context"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// cancellationSlotProbe fills the real registry's only slot, then queues a
// distinct command. The first waiting diagnostic publishes readiness; neither
// an elapsed delay nor a marker written before admission stands in for waiting.
func cancellationSlotProbe(ready string) (any, int) {
	if err := env.Set(job.ParentKey, ""); err != nil {
		return probeText(err.Error()), 2
	}
	if err := env.Set(job.SlotsKey, "1"); err != nil {
		return probeText(err.Error()), 2
	}
	root, err := lepath.Root()
	if err != nil {
		return probeText(err.Error()), 2
	}
	adm, err := job.NewIn(root)
	if err != nil {
		return probeText(err.Error()), 2
	}
	held, err := adm.AdmitContext(context.Background(), "owned-holder", []string{"fixture-holder"})
	if err != nil {
		return probeText(err.Error()), 2
	}
	defer held.Release(1)
	adm.Err = cancellationQueueWriter{ready: ready}
	_, code := adm.RunContext(context.Background(), "owned-waiter", []string{"sh", "-c", "touch \"$1\"", "sh", ready + ".ran"}, root, nil)
	return nil, code
}

type cancellationQueueWriter struct{ ready string }

func (w cancellationQueueWriter) Write(body []byte) (int, error) {
	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(w.ready, []byte(pid+" "+pid+"\n"), 0o600); err != nil {
		return 0, err
	}
	return os.Stderr.Write(body)
}
