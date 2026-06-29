// VALIDATES: ISSUE-2 (bounded poll for partition node after BLKRRPART)
// PREVENTS: race where mount of part-4 fails because devtmpfs has not yet created the node

package disk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForPartitionNodeAppears(t *testing.T) {
	dir := t.TempDir()
	node := filepath.Join(dir, "sda4")

	go func() {
		time.Sleep(100 * time.Millisecond)
		f, err := os.Create(node)
		if err != nil {
			t.Error(err)
			return
		}
		f.Close() //nolint:errcheck // test
	}()

	origInterval := partPollInterval
	partPollInterval = 50 * time.Millisecond
	defer func() { partPollInterval = origInterval }()

	err := waitForPartitionNode(node, 2*time.Second)
	if err != nil {
		t.Fatalf("waitForPartitionNode: %v", err)
	}
}

func TestWaitForPartitionNodeTimeout(t *testing.T) {
	node := "/dev/nonexistent-test-partition-xyz"

	origInterval := partPollInterval
	partPollInterval = 50 * time.Millisecond
	defer func() { partPollInterval = origInterval }()

	err := waitForPartitionNode(node, 200*time.Millisecond)
	if err == nil {
		t.Fatal("waitForPartitionNode should fail when node never appears")
	}
}
