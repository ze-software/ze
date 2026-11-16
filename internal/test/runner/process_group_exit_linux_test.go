// Design: docs/architecture/testing/runner-architecture.md -- observe group exit before reaping its leader.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLinuxProcessGroupLive selects the pinned group and distinguishes its
// live members from zombies, unrelated groups and already-disappeared tasks.
func TestLinuxProcessGroupLive(t *testing.T) {
	for _, tc := range []struct {
		name string
		stat string
		live bool
	}{
		{name: "live member", stat: "42 (sleep) S 1 41", live: true},
		{name: "exited member", stat: "42 (sleep) Z 1 41"},
		{name: "unrelated live member", stat: "42 (sleep) S 1 42"},
		{name: "parenthesized command", stat: "42 (a ) command) S 1 41", live: true},
		{name: "disappeared task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// /proc also contains nonnumeric entries such as self and net.
			if err := os.WriteFile(filepath.Join(root, "self"), []byte("not a numeric task"), 0o600); err != nil {
				t.Fatal(err)
			}
			writeLinuxGroupMember(t, root, "41", "41 (leader) Z 1 41")
			member := filepath.Join(root, "42")
			if err := os.Mkdir(member, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.stat != "" {
				writeLinuxGroupMember(t, root, "42", tc.stat)
			}
			live, err := linuxProcessGroupLive(root, 41)
			if err != nil {
				t.Fatal(err)
			}
			if live != tc.live {
				t.Fatalf("group live=%v, want %v", live, tc.live)
			}
		})
	}
}

// TestLinuxProcessGroupRequiresHeldLeader refuses an empty process table or
// an identity mismatch; neither proves exit of the still-unreaped group.
func TestLinuxProcessGroupRequiresHeldLeader(t *testing.T) {
	for _, stat := range []string{"", "41 (leader) Z 1 42"} {
		root := t.TempDir()
		if stat != "" {
			leader := filepath.Join(root, "41")
			if err := os.Mkdir(leader, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(leader, "stat"), []byte(stat), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := linuxProcessGroupLive(root, 41); err == nil {
			t.Fatalf("missing or mismatched held leader was accepted: %q", stat)
		}
	}
}

// TestLinuxProcessGroupUnreadableMember rejects a numeric task whose state
// cannot be read; that failure must never mean that the group has exited.
func TestLinuxProcessGroupUnreadableMember(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "41", "stat"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := linuxProcessGroupLive(root, 41); err == nil {
		t.Fatal("unreadable member was accepted as an exited group")
	}
}

// TestWaitLinuxProcessGroupKeepsUnknownAsFailure refuses a missing or malformed
// process table and bounds a member that has not reached an exited state.
func TestWaitLinuxProcessGroupKeepsUnknownAsFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		stat string
		want string
	}{
		{name: "missing process table", want: "no such file"},
		{name: "invalid command", stat: "invalid", want: "invalid process state"},
		{name: "incomplete state", stat: "41 (sleep) S", want: "incomplete process state"},
		{name: "invalid state", stat: "41 (sleep) ZZ 1 41", want: "invalid process state"},
		{name: "invalid group", stat: "41 (sleep) S 1 invalid", want: "read process group"},
		{name: "live member deadline", stat: "41 (sleep) S 1 41", want: "still has live members"},
		{name: "exited member", stat: "41 (sleep) Z 1 41"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "proc")
			if tc.stat != "" {
				writeLinuxGroupMember(t, root, "41", tc.stat)
			}
			err := waitLinuxProcessGroup(root, 41, time.Millisecond)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("group observation succeeded, want %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("group observation=%v, want %q", err, tc.want)
			}
		})
	}
}

// TestLinuxProcessGroupZombieThreads requires a zombie member's lone task to
// retain the expected identity. Missing task information never proves exit.
func TestLinuxProcessGroupZombieThreads(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tasks []string
		stat  string
		live  bool
		want  string
	}{
		{name: "lone zombie", tasks: []string{"42"}, stat: "42 (member) Z 1 41"},
		{name: "surviving thread", tasks: []string{"42", "43"}, stat: "42 (member) Z 1 41", live: true},
		{name: "leader still live", tasks: []string{"42"}, stat: "42 (member) S 1 41", live: true},
		{name: "empty task table", tasks: []string{}, want: "has no leader"},
		{name: "missing task table", want: "no such file"},
		{name: "missing task leader", tasks: []string{"43"}, want: "has no leader"},
		{name: "unreadable task leader", tasks: []string{"42"}, want: "no such file"},
		{name: "wrong task group", tasks: []string{"42"}, stat: "42 (member) Z 1 99", want: "want 41"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLinuxGroupMember(t, root, "41", "41 (leader) Z 1 41")
			member := filepath.Join(root, "42")
			if err := os.Mkdir(member, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(member, "stat"), []byte("42 (member) Z 1 41"), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.tasks != nil {
				if err := os.Mkdir(filepath.Join(member, "task"), 0o700); err != nil {
					t.Fatal(err)
				}
				for _, task := range tc.tasks {
					if err := os.Mkdir(filepath.Join(member, "task", task), 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.stat != "" {
				if err := os.WriteFile(filepath.Join(member, "task", "42", "stat"), []byte(tc.stat), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			live, err := linuxProcessGroupLive(root, 41)
			if tc.want != "" {
				if err == nil {
					t.Fatalf("task observation succeeded, want %q", tc.want)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("task observation=%v, want %q", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if live != tc.live {
				t.Fatalf("group live=%v, want %v", live, tc.live)
			}
		})
	}
}

// TestLinuxProcessTaskDisappearance distinguishes an exited descendant from a
// missing pinned parent and a still-present member with an unreadable task table.
func TestLinuxProcessTaskDisappearance(t *testing.T) {
	root := t.TempDir()
	taskErr := os.ErrPermission
	exited, err := linuxProcessDisappearance(root, "42", 41, taskErr)
	if err != nil {
		t.Fatal(err)
	}
	if !exited {
		t.Fatal("verified descendant disappearance was not accepted")
	}
	if _, err := linuxProcessDisappearance(root, "41", 41, taskErr); err == nil {
		t.Fatal("missing pinned leader was accepted as an exited descendant")
	}
	if err := os.Mkdir(filepath.Join(root, "42"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := linuxProcessDisappearance(root, "42", 41, taskErr); err == nil {
		t.Fatal("unreadable task table was accepted while its process remains")
	}
}

func writeLinuxGroupMember(t *testing.T, root, pid, stat string) {
	t.Helper()
	member := filepath.Join(root, pid)
	task := filepath.Join(member, "task", pid)
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(member, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}
}
