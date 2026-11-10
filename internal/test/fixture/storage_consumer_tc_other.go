// Design: docs/architecture/traffic/tc-original-qdisc-restore.md -- real kernel restart evidence.
// Related: storage_consumer_tc.go -- the Linux consumer this refusal stands in for

//go:build !linux

package fixture

import (
	"context"
	"errors"
)

// storageTCRestart refuses off Linux: the tc consumer programs a root qdisc
// through netlink, which only Linux has. test/traffic/storage-tc-restart.ci
// declares needs-linux, so the runner skips it before this answers.
func storageTCRestart(context.Context, string) error {
	return errors.New("storage/consumer-restart-tc programs a qdisc through netlink, which only Linux has: run it in the QEMU guest with ./le test qemu all test test/traffic/storage-tc-restart.ci")
}
