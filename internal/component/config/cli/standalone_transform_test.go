// Design: docs/architecture/api/commands.md -- detached transformations preserve publication targets.

package cli

import (
	"os"
	"strings"
	"testing"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/redact"
)

// TestSuppliedConfigDetachedMutationsMatchFileCommands compares actual file
// invocation results with returned detached bytes, without changing the source.
func TestSuppliedConfigDetachedMutationsMatchFileCommands(t *testing.T) {
	for _, operation := range []string{"set", "deactivate", "activate"} {
		t.Run(operation, func(t *testing.T) {
			content := showTestConfig
			if operation == "activate" {
				content = strings.Replace(content, "router-id 1.2.3.4;", "inactive: router-id 1.2.3.4;", 1)
			}
			file := writeTestConfig(t, content)
			var code int
			if operation == "set" {
				code = cmdSet([]string{file, "bgp", "router-id", "5.6.7.8"})
			} else {
				code = runDeactivateLike(nil, []string{file, "bgp", "router-id"}, operation == "activate")
			}
			if code != exitOK {
				t.Fatalf("local mutation failed: %d", code)
			}
			expected, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if string(expected) == content {
				t.Fatal("local positive control did not change the source")
			}
			statement := operation + " bgp router-id"
			if operation == "set" {
				statement += " 5.6.7.8"
			}
			tokens := suppliedConfigTokens(suppliedConfigPair(t, content, statement))
			answer := suppliedConfigAnswer(t, operation, tokens)
			blob, ok := answer.Payload.(command.Blob)
			if answer.Code != exitOK || !ok {
				t.Fatalf("detached mutation refused: %+v", answer)
			}
			if string(blob) != string(expected) {
				t.Fatalf("detached mutation differs from file command:\n%s\nwant:\n%s", blob, expected)
			}
		})
	}
}

// TestSuppliedConfigReplacementRetainsValidationWarnings compares a warning-only
// replacement with the transition validator, for both serialized and diff output.
func TestSuppliedConfigReplacementRetainsValidationWarnings(t *testing.T) {
	// The canonical password warns before Save replaces it with the requested
	// plaintext companion's hash. The companion also produces a Save advisory.
	const replacement = validConfig + `
system {
    authentication {
        user alice {
            password old-clear-value;
            plaintext-password secret;
        }
    }
}
`
	validator, err := editor.NewConfigValidator()
	if err != nil {
		t.Fatal(err)
	}
	validation := validator.ValidateTransition(validConfig, replacement)
	if validation.HasErrors() || !validation.HasWarnings() {
		t.Fatalf("warning-only positive control failed: %+v", validation)
	}
	for _, preview := range []bool{false, true} {
		tokens := suppliedConfigTokens(suppliedConfigPair(t, validConfig, replacement))
		if preview {
			tokens = append(tokens, "dry-run")
		}
		answer := suppliedConfigAnswer(t, "replace", tokens)
		if answer.Code != exitOK || answer.Payload == nil {
			t.Fatalf("warning-only replacement refused: %+v", answer)
		}
		for _, warning := range validation.Warnings {
			if !strings.Contains(answer.Diagnostic, warning.Text(nil)) {
				t.Fatalf("preview=%t lost validation warning %q: %s", preview, warning.Text(nil), answer.Diagnostic)
			}
		}
		if !preview && !strings.Contains(answer.Diagnostic, "weak password (shorter than 8 characters)") {
			t.Fatalf("replacement lost the Save advisory: %s", answer.Diagnostic)
		}
	}
}

// TestSuppliedConfigDetachedMutationRefusals checks schema value and source
// path boundaries before any transformed document is offered for download.
func TestSuppliedConfigDetachedMutationRefusals(t *testing.T) {
	for _, test := range []struct {
		operation string
		statement string
	}{
		{operation: "set", statement: "set bgp router-id not-an-address"},
		{operation: "deactivate", statement: "deactivate bgp absent"},
		{operation: "activate", statement: "activate bgp absent"},
	} {
		t.Run(test.operation, func(t *testing.T) {
			answer := suppliedConfigAnswer(t, test.operation, suppliedConfigTokens(suppliedConfigPair(t, showTestConfig, test.statement)))
			if answer.Code == exitOK || answer.Payload != nil || answer.Diagnostic == "" {
				t.Fatalf("invalid mutation answered a complete document: %+v", answer)
			}
		})
	}
}

// TestSuppliedConfigReplacementRemainsDetached checks replacement uses the
// editor's serializer but never writes a label chosen by the caller.
func TestSuppliedConfigReplacementRemainsDetached(t *testing.T) {
	file := writeTestConfig(t, validConfig)
	changed := strings.Replace(validConfig, "203.0.113.253", "5.6.7.8", 1)
	content := suppliedConfigPair(t, validConfig, changed)
	answer := suppliedConfigAnswer(t, "replace", suppliedConfigTokens(content))
	blob, ok := answer.Payload.(command.Blob)
	if answer.Code != exitOK || !ok {
		t.Fatalf("replacement refused: %+v", answer)
	}
	if !strings.Contains(string(blob), "5.6.7.8") {
		t.Fatalf("replacement was not serialized: %s", blob)
	}
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != validConfig {
		t.Fatal("detached replacement changed the caller file")
	}
	answer = suppliedConfigAnswer(t, "replace", suppliedConfigTokens(suppliedConfigPair(t, validConfig, "invalid syntax")))
	if answer.Code == exitOK || answer.Payload != nil {
		t.Fatalf("invalid replacement answered output: %+v", answer)
	}
}

// TestSuppliedConfigDetachedStatementsAreSingleCommands exercises the parser
// boundary with commands that could otherwise dispatch or change more than one
// setting. The entire assigned value remains inside the redacted envelope.
func TestSuppliedConfigDetachedStatementsAreSingleCommands(t *testing.T) {
	for _, statement := range []string{
		"update data key meta/test content eA==",
		"set bgp router-id 5.6.7.8\nset bgp router-id 9.9.9.9",
		"set bgp router-id 5.6.7.8 | json",
		"set",
	} {
		answer := suppliedConfigAnswer(t, "set", suppliedConfigTokens(suppliedConfigPair(t, showTestConfig, statement)))
		if answer.Code == exitOK {
			t.Fatalf("non-single set command was accepted: %s", statement)
		}
		if answer.Payload != nil {
			t.Fatal("refused command retained transformed bytes")
		}
	}
	const credential = "keep-this-value-out-of-history"
	tokens := suppliedConfigTokens(suppliedConfigPair(t, showTestConfig, "set bgp peer peer1 description "+credential))
	line := "show config supplied set " + strings.Join(tokens, " ")
	if strings.Contains(redact.Command(line), tokens[1]) {
		t.Fatal("command accounting retained the encoded assignment")
	}
	answer := suppliedConfigAnswer(t, "set", tokens)
	if answer.Code != exitOK {
		t.Fatalf("ordinary bounded single-statement control was refused: %s", answer.Diagnostic)
	}
}
