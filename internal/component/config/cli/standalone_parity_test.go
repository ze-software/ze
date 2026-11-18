// Design: docs/architecture/api/commands.md -- standalone forms share supplied producers.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

func init() {
	env.MustRegister(env.EnvEntry{Key: "ze.test.config.parity.duration", Type: env.TypeDuration,
		Description: "Duration used by configuration environment validation tests"})
}

// TestSuppliedConfigMigrationForms exercises both modeled output choices through
// the registered remote handler and compares the existing local converter.
func TestSuppliedConfigMigrationForms(t *testing.T) {
	file := writeTestConfig(t, showTestConfig)
	for _, form := range []string{formSet, formHierarchical} {
		t.Run(form, func(t *testing.T) {
			want, _, err := configMigrateWithWarnings(file, "", form)
			if err != nil {
				t.Fatal(err)
			}
			tokens := append(suppliedConfigTokens(showTestConfig), "format", form)
			answer := suppliedConfigAnswer(t, "migrate", tokens)
			if answer.Code != exitOK {
				t.Fatalf("migration refused: %s", answer.Diagnostic)
			}
			blob, ok := answer.Payload.(command.Blob)
			if !ok {
				t.Fatalf("migration answer is %T, want Blob", answer.Payload)
			}
			if string(blob) != want {
				t.Fatalf("migration differs from local producer:\n%s\nwant:\n%s", blob, want)
			}
		})
	}
}

// TestSuppliedConfigFixPlanKeepsDiagnostics checks that a broken source still
// answers the same plan-only diagnostic document as the local command.
func TestSuppliedConfigFixPlanKeepsDiagnostics(t *testing.T) {
	const content = "invalid config syntax"
	const name = "broken.conf"
	want := diagnostic.NewFixPlan(name, runValidation(content, name).Diagnostics)
	tokens := append(suppliedConfigTokens(content), "name", name)
	answer := suppliedConfigAnswer(t, "fix-plan", tokens)
	if answer.Code != exitOK {
		t.Fatalf("plan refused: %s", answer.Diagnostic)
	}
	got, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("plan differs: %s; want %s", got, expected)
	}
}

// TestSuppliedConfigSourceSubtreeRetainsMasking checks whole and nested source
// displays, and verifies a missing path cannot silently return the whole file.
func TestSuppliedConfigSourceSubtreeRetainsMasking(t *testing.T) {
	file := writeTestConfig(t, showTestConfig)
	for _, path := range []string{"", "bgp peer peer1"} {
		t.Run(path, func(t *testing.T) {
			var expected bytes.Buffer
			args := append([]string{file}, strings.Fields(path)...)
			if code := showConfig(&expected, args); code != exitOK {
				t.Fatalf("local source control failed: %d", code)
			}
			tokens := suppliedConfigTokens(showTestConfig)
			if path != "" {
				tokens = append(tokens, "path", path)
			}
			answer := suppliedConfigAnswer(t, "source", tokens)
			blob, ok := answer.Payload.(command.Blob)
			if answer.Code != exitOK || !ok || string(blob) != expected.String() {
				t.Fatalf("source differs: code=%d payload=%v diagnostic=%s", answer.Code, answer.Payload, answer.Diagnostic)
			}
		})
	}
	answer := suppliedConfigAnswer(t, "source", append(suppliedConfigTokens(showTestConfig), "path", "bgp peer missing"))
	if answer.Code == exitOK || answer.Payload != nil {
		t.Fatalf("missing source path answered data: %+v", answer)
	}
	answer = suppliedConfigAnswer(t, "source", suppliedConfigTokens(testConfigSecretBase))
	if strings.Contains(encodedAnswer(t, answer), diffStoredPassword) {
		t.Fatal("source display revealed a sensitive value")
	}
}

// TestConfigEnvironmentValidationRejectsMalformedValue checks the CLI's
// environment-only option against a registered type before and after repair.
func TestConfigEnvironmentValidationRejectsMalformedValue(t *testing.T) {
	t.Setenv("ze.test.config.parity.duration", "not-a-duration")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if code := cmdValidate([]string{"--limit", "environment", "-q"}); code != exitInvalid {
		t.Fatalf("malformed environment answered code %d, want invalid", code)
	}
	t.Setenv("ze.test.config.parity.duration", "5s")
	env.ResetCache()
	if code := cmdValidate([]string{"--limit", "environment", "-q"}); code != exitOK {
		t.Fatalf("corrected environment answered code %d", code)
	}
}

// TestSuppliedConfigMigrationBoundsExpandedOutput uses a real nested source
// whose repeated set paths amplify a bounded input beyond the remote limit.
// The same large source remains available to the local converter.
func TestSuppliedConfigMigrationBoundsExpandedOutput(t *testing.T) {
	var source strings.Builder
	source.WriteString("bgp { group ")
	source.WriteString(strings.Repeat("g", 128<<10))
	source.WriteString(" {\n")
	for index := range 64 {
		fmt.Fprintf(&source, "peer p%d { description edge; }\n", index)
	}
	source.WriteString("} }\n")
	content := source.String()
	if len(content) > command.ContentOctetsMax {
		t.Fatal("amplification fixture exceeds the input bound")
	}
	file := writeTestConfig(t, content)
	local, _, err := configMigrateWithWarnings(file, "", formSet)
	if err != nil {
		t.Fatal(err)
	}
	if len(local) <= command.BlobRemoteOctetsMax {
		t.Fatal("amplification fixture does not exceed the produced-byte bound")
	}
	answer := suppliedConfigAnswer(t, "migrate", suppliedConfigTokens(content))
	if answer.Code == exitOK {
		t.Fatal("oversized generated set output was accepted remotely")
	}
	if answer.Payload != nil {
		t.Fatal("remote refusal retained an oversized payload")
	}
	if !strings.Contains(answer.Diagnostic, "local ze config") {
		t.Fatalf("oversized output has no local recovery: %s", answer.Diagnostic)
	}
	answer = suppliedConfigAnswer(t, "migrate", append(suppliedConfigTokens(content), "format", formHierarchical))
	if answer.Code != exitOK {
		t.Fatalf("bounded hierarchical output was refused: %s", answer.Diagnostic)
	}
}

// TestStandaloneConfigFileToolsConfineRemotePaths exercises real file
// registrations, including the raw-source fallback's secret boundary.
func TestStandaloneConfigFileToolsConfineRemotePaths(t *testing.T) {
	inside := confinedConfigDir(t, showTestConfig)
	outside := writeTestConfig(t, showTestConfig)
	for _, operation := range []string{"migrate", "fix-plan", "source"} {
		t.Run(operation, func(t *testing.T) {
			answer := answerOn(t, "show config "+operation, []string{"file", inside}, true)
			if answer.Code != exitOK {
				t.Fatalf("confined file refused: %s", answer.Diagnostic)
			}
			answer = answerOn(t, "show config "+operation, []string{"file", outside}, true)
			if answer.Code == exitOK {
				t.Fatal("unconfined filename was read remotely")
			}
			if answer.Payload != nil {
				t.Fatal("filename refusal retained source bytes")
			}
		})
	}
	if err := os.WriteFile(inside, []byte("invalid syntax with a private value"), 0o600); err != nil {
		t.Fatal(err)
	}
	answer := answerOn(t, "show config source", []string{"file", inside}, true)
	if answer.Code == exitOK {
		t.Fatal("unparseable daemon file was returned without secret masking")
	}
	answer = suppliedConfigAnswer(t, "source", suppliedConfigTokens("invalid syntax with a private value"))
	if answer.Code != exitOK {
		t.Fatal("exact caller-supplied source lost its repair display")
	}
}
