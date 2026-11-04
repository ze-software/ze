// Design: docs/contributing/spec-workflow.md -- exemptions belong to canonical subjects
// VALIDATES: the registered design gate judges canonical production targets,
// including test-named and unsupported-extension aliases, without losing genuine exemptions.
// PREVENTS: lexical filenames erasing producer obligations before identity is resolved.
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canonicalProducer = "internal/exabgp/bridge/bridge_status.go"
const canonicalProducerTest = "internal/exabgp/bridge/bridge_status_test.go"
const canonicalUnrelated = "internal/control/producer.go"

func canonicalSubjectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeHookFixture(t, root, canonicalProducer, "package bridge\n")
	writeHookFixture(t, root, canonicalProducerTest, "package bridge\n")
	writeHookFixture(t, root, canonicalUnrelated, "package control\n")
	writeHookFixture(t, root, "docs/producer-note.md", "# Producer note\n")
	if err := os.MkdirAll(filepath.Join(root, "internal/le"), 0o750); err != nil {
		t.Fatal(err)
	}
	for alias, target := range map[string]string{
		"internal/le/producer_test.go": canonicalProducer,
		"internal/le/producer.txt":     canonicalProducer,
		"internal/le/test_alias.go":    canonicalProducerTest,
		"internal/le/note_alias.go":    "docs/producer-note.md",
	} {
		if err := os.Symlink(filepath.Join(root, filepath.FromSlash(target)), filepath.Join(root, filepath.FromSlash(alias))); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDesignEvidenceExemptionsFollowCanonicalProducer(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify", "Files to Create"} {
		for _, subject := range []struct {
			name     string
			path     string
			producer bool
		}{
			{"canonical-production", canonicalProducer, true},
			{"test-named-alias-to-production", "internal/le/producer_test.go", true},
			{"unsupported-extension-alias-to-production", "internal/le/producer.txt", true},
			{"genuine-canonical-test", canonicalProducerTest, false},
			{"production-looking-alias-to-genuine-test", "internal/le/test_alias.go", false},
		} {
			for _, matching := range []bool{false, true} {
				state := "unrelated-read"
				if matching {
					state = "matching-current-target-read"
				}
				t.Run(heading+"/"+subject.name+"/"+state, func(t *testing.T) {
					root := canonicalSubjectFixture(t)
					readPath, content := canonicalUnrelated, "package control\n"
					if matching {
						readPath, content = canonicalProducer, "package bridge\n"
					}
					recordObservedRead(t, root, filepath.Join(root, filepath.FromSlash(readPath)), content, 1, 2, 2)
					want := 0
					if subject.producer && !matching {
						want = 2
					}
					message := observedDesignGate(t, root, "## "+heading+"\n`"+subject.path+"`\n", want)
					if want == 2 && !strings.Contains(message, canonicalProducer) {
						t.Fatalf("refusal does not name canonical production subject: %s", message)
					}
				})
			}
		}
	}
}

func TestDesignEvidenceExemptAliasesRefuseUnknownIdentity(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify", "Files to Create"} {
		for _, suffix := range []string{"_test.go", ".txt"} {
			for _, failure := range []string{"dangling", "outside", "permission-denied"} {
				t.Run(heading+"/"+suffix+"/"+failure, func(t *testing.T) {
					root := canonicalSubjectFixture(t)
					recordObservedRead(t, root, filepath.Join(root, filepath.FromSlash(canonicalUnrelated)), "package control\n", 1, 2, 2)
					alias := "internal/aliases/producer" + suffix
					aliasDir := filepath.Join(root, "internal/aliases")
					if err := os.MkdirAll(aliasDir, 0o750); err != nil {
						t.Fatal(err)
					}
					target := filepath.Join(root, "internal/missing/producer.go")
					if failure == "outside" {
						outside := t.TempDir()
						writeHookFixture(t, outside, "producer.go", "package outside\n")
						target = filepath.Join(outside, "producer.go")
					} else if failure == "permission-denied" {
						target = filepath.Join(root, filepath.FromSlash(canonicalProducer))
					}
					if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(alias))); err != nil {
						t.Fatal(err)
					}
					if failure == "permission-denied" {
						if err := os.Chmod(aliasDir, 0); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if err := os.Chmod(aliasDir, 0o750); err != nil {
								t.Error(err)
							}
						})
						if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(alias))); !os.IsPermission(err) {
							t.Skip("the current user can traverse a mode-000 directory; permission-denied fixture is unavailable")
						}
					}
					// This producer stays readable outside the denied/escaped alias subtree.
					observedDesignGate(t, root, "## Files to Modify\n`"+canonicalUnrelated+"`\n", 0)
					message := observedDesignGate(t, root, "## "+heading+"\n`"+alias+"`\n", 2)
					if !strings.Contains(message, alias) {
						t.Fatalf("unknown canonical subject is not named: %s", message)
					}
				})
			}
		}
	}
}

func TestDesignEvidenceCanonicalExemptionsPreserveNewFiles(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{"genuinely-new-source", "internal/new/producer.go"},
		{"genuinely-new-test", "internal/new/producer_test.go"},
		{"genuinely-new-note", "docs/new/producer.txt"},
		{"existing-canonical-test", canonicalProducerTest},
		{"existing-canonical-note", "docs/producer-note.md"},
		{"production-looking-alias-to-note", "internal/le/note_alias.go"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := canonicalSubjectFixture(t)
			recordObservedRead(t, root, filepath.Join(root, filepath.FromSlash(canonicalUnrelated)), "package control\n", 1, 2, 2)
			observedDesignGate(t, root, "## Files to Create\n`"+test.path+"`\n", 0)
		})
	}
}
