// Design: docs/architecture/api/commands.md -- caller-supplied configuration inspection.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// suppliedConfigAnswer uses the same registrations and model as local and
// daemon dispatch. A missing owner or model is a behavioral failure, not a
// dependency on an API that the implementation has not introduced yet.
func suppliedConfigAnswer(t *testing.T, operation string, tokens []string) command.LocalAnswer {
	t.Helper()
	return suppliedConfigAnswerContext(t, context.Background(), operation, tokens)
}

func suppliedConfigAnswerContext(t *testing.T, ctx context.Context, operation string, tokens []string) command.LocalAnswer {
	t.Helper()
	path := "show config supplied " + operation
	handler, rest := command.LookupLocalData(strings.Fields(path))
	if handler == nil || len(rest) != 0 {
		t.Fatalf("%s has no exact owned local-data handler", path)
	}
	validated, err := command.ValidateModelArgs(path, tokens, nil)
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}
	return command.AnswerRemote(handler, validated.WithContext(ctx))
}

func suppliedConfigTokens(content string) []string {
	return []string{"content", base64.StdEncoding.EncodeToString([]byte(content))}
}

func suppliedConfigPair(t *testing.T, before, after string) string {
	t.Helper()
	data, err := json.Marshal(map[string]string{"before": before, "after": after})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestSuppliedConfigSingleInputsMatchLocalReaders proves all four single-input
// daemon forms answer the existing byte producer's result, including the
// validation verdict and its diagnostics. No caller file is sent as a path.
func TestSuppliedConfigSingleInputsMatchLocalReaders(t *testing.T) {
	file := writeTestConfig(t, validConfig)
	for _, operation := range []string{"dump", "fmt", "graph"} {
		t.Run(operation, func(t *testing.T) {
			local := answerOn(t, "show config "+operation, []string{file}, false)
			if local.Code != 0 || local.Payload == nil {
				t.Fatalf("local positive control failed: code %d, %s", local.Code, local.Diagnostic)
			}
			got := suppliedConfigAnswer(t, operation, suppliedConfigTokens(validConfig))
			if encodedAnswer(t, got) != encodedAnswer(t, local) {
				t.Errorf("supplied answer differs from the local producer:\n%s\nwant:\n%s",
					encodedAnswer(t, got), encodedAnswer(t, local))
			}
		})
	}
	t.Run("validation", func(t *testing.T) {
		const label = "uploaded.conf"
		result := runValidation(validConfig, label)
		if !result.Valid {
			t.Fatalf("validation positive control is invalid: %+v", result.Diagnostics)
		}
		want := command.LocalAnswer{Payload: diagnostic.NewValidateResult(
			result.Path, result.Valid, result.Diagnostics, result.Config)}
		tokens := append(suppliedConfigTokens(validConfig), "name", label)
		got := suppliedConfigAnswer(t, "validation", tokens)
		if encodedAnswer(t, got) != encodedAnswer(t, want) {
			t.Errorf("supplied validation differs from its shared producer:\n%s\nwant:\n%s",
				encodedAnswer(t, got), encodedAnswer(t, want))
		}
	})
}

// TestSuppliedConfigFormattingSelectionsMatchLocalProducer checks both
// formatting choices, including check's nonzero verdict without a payload.
func TestSuppliedConfigFormattingSelectionsMatchLocalProducer(t *testing.T) {
	file := writeTestConfig(t, validConfig)
	label := filepath.Base(file)
	for _, selection := range []string{"check", "diff"} {
		t.Run(selection, func(t *testing.T) {
			want := answerOn(t, "show config fmt", []string{selection, file}, false)
			want.Diagnostic = strings.ReplaceAll(want.Diagnostic, file, label)
			if want.Diagnostic == "" {
				t.Fatal("unformatted local control gave no check verdict or diff")
			}
			tokens := append(suppliedConfigTokens(validConfig), "name", label, selection)
			got := suppliedConfigAnswer(t, "fmt", tokens)
			if encodedAnswer(t, got) != encodedAnswer(t, want) {
				t.Errorf("supplied formatting choice differs from local producer:\n%s\nwant:\n%s",
					encodedAnswer(t, got), encodedAnswer(t, want))
			}
		})
	}
}

// TestSuppliedConfigDumpMasksSecrets checks both supported display choices at
// the answer producer, before an adapter or pipe can see the payload.
func TestSuppliedConfigDumpMasksSecrets(t *testing.T) {
	for _, strip := range []bool{false, true} {
		t.Run(map[bool]string{false: "encoded", true: "stripped"}[strip], func(t *testing.T) {
			tokens := suppliedConfigTokens(testConfigSecretBase)
			if strip {
				tokens = append(tokens, "strip-private")
			}
			got := suppliedConfigAnswer(t, "dump", tokens)
			if got.Code != 0 || got.Payload == nil {
				t.Fatalf("supplied dump refused: code %d, %s", got.Code, got.Diagnostic)
			}
			data := encodedAnswer(t, got)
			if !strings.Contains(data, "password") || strings.Contains(data, diffStoredPassword) {
				t.Errorf("dump lost the password leaf or disclosed its plaintext: %s", data)
			}
			if strip && !strings.Contains(data, config.SecretDataPlaceholder) {
				t.Errorf("strip-private did not use the existing placeholder: %s", data)
			}
		})
	}
}

// TestSuppliedConfigDiffMatchesLocalComparison proves both inputs participate
// in the existing resolved comparison, with the existing secret masking.
func TestSuppliedConfigDiffMatchesLocalComparison(t *testing.T) {
	before := writeTestConfig(t, testConfigSecretBase)
	after := writeTestConfig(t, testConfigSecretRotated)
	want := answerOn(t, "show config diff", []string{before, after}, false)
	if want.Code != 0 || want.Payload == nil {
		t.Fatalf("local comparison control failed: code %d, %s", want.Code, want.Diagnostic)
	}
	got := suppliedConfigAnswer(t, "diff", suppliedConfigTokens(
		suppliedConfigPair(t, testConfigSecretBase, testConfigSecretRotated)))
	if encodedAnswer(t, got) != encodedAnswer(t, want) {
		t.Errorf("supplied diff differs from the local comparison:\n%s\nwant:\n%s",
			encodedAnswer(t, got), encodedAnswer(t, want))
	}
	for _, secret := range []string{diffStoredPassword, diffRotatedPassword} {
		if strings.Contains(encodedAnswer(t, got), secret) {
			t.Errorf("supplied comparison disclosed a source credential")
		}
	}
}

// TestSuppliedConfigValidationRetainsInvalidEvidence proves a typed refusal
// still carries its diagnostic payload, rather than becoming a transport error
// that loses the explanation of the invalid source.
func TestSuppliedConfigValidationRetainsInvalidEvidence(t *testing.T) {
	got := suppliedConfigAnswer(t, "validation", append(
		suppliedConfigTokens("bgp { router-id invalid-address; }"), "name", "invalid.conf"))
	if got.Code == 0 || got.Payload == nil || got.Diagnostic == "" {
		t.Fatalf("invalid supplied source lost its verdict or evidence: %+v", got)
	}
	var result diagnostic.ValidateResult
	data, err := json.Marshal(got.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Valid || result.Path != "invalid.conf" || len(result.Diagnostics) == 0 {
		t.Errorf("invalid supplied source has the wrong validation evidence: %+v", result)
	}
}

// TestSuppliedConfigDiffRefusesAmbiguousEnvelopes prevents accepting an
// accidentally omitted source, or silently choosing one of two duplicate
// values. These are valid base64, so the envelope producer must refuse them.
func TestSuppliedConfigDiffRefusesAmbiguousEnvelopes(t *testing.T) {
	for name, envelope := range map[string]string{
		"missing before":    `{"after":""}`,
		"missing after":     `{"before":""}`,
		"duplicate before":  `{"before":"","before":"# hidden","after":""}`,
		"duplicate after":   `{"before":"","after":"","after":"# hidden"}`,
		"unknown field":     `{"before":"","after":"","file":"/etc/passwd"}`,
		"wrong source type": `{"before":23,"after":""}`,
		"null source":       `{"before":null,"after":""}`,
		"trailing object":   `{"before":"","after":""}{}`,
		"not an object":     `["",""]`,
		"malformed JSON":    `{"before":`,
		"invalid field":     `{?}`,
		"missing close":     `{"before":"","after":""`,
		"invalid UTF-8":     string([]byte{'{', '"', 0xff, '"', ':', '"', '"', '}'}),
	} {
		t.Run(name, func(t *testing.T) {
			got := suppliedConfigAnswer(t, "diff", suppliedConfigTokens(envelope))
			if got.Code == 0 || got.Payload != nil || got.Diagnostic == "" {
				t.Errorf("ambiguous comparison envelope was not refused: %+v", got)
			}
		})
	}
}

// TestSuppliedConfigDeclaredContentBounds proves the model, which every command
// surface reads, accepts exactly the existing single SSH packet budget. Diff's
// limit is on the combined envelope, not separately on each source.
func TestSuppliedConfigDeclaredContentBounds(t *testing.T) {
	const decodedMax = 195840
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			const paddingMarker = "BOUNDARY_PADDING"
			// Keep native comment spacing; a hash without a space denotes set metadata.
			content := validConfig + "\n# " + paddingMarker + "\n"
			if operation == "diff" {
				content = suppliedConfigPair(t, content, validConfig)
			}
			// Pad after JSON encoding so diff's combined decoded envelope meets the same bound.
			content = strings.Replace(content, paddingMarker, strings.Repeat("x", decodedMax-len(content)+len(paddingMarker)), 1)
			if len(content) != decodedMax {
				t.Fatalf("boundary fixture has %d decoded bytes, want %d", len(content), decodedMax)
			}
			tokens := suppliedConfigTokens(content)
			path := "show config supplied " + operation
			if _, err := command.ValidateModelArgs(path, tokens, nil); err != nil {
				t.Fatalf("model refused the exact packet bound: %v", err)
			}
			got := suppliedConfigAnswer(t, operation, tokens)
			if got.Code != 0 || got.Payload == nil {
				t.Fatalf("exact packet bound refused: code %d, %s", got.Code, got.Diagnostic)
			}
			tooLong := base64.StdEncoding.EncodeToString([]byte(content + " "))
			_, err := command.ValidateModelArgs(path, []string{"content", tooLong}, nil)
			if err == nil {
				t.Fatal("model accepted content beyond one SSH packet")
			}
			if strings.Contains(err.Error(), tooLong) {
				t.Fatal("the bound refusal disclosed encoded source content")
			}
		})
	}
}

// TestSuppliedConfigRefusesInvalidArguments checks failures before parsing,
// including replaying the history placeholder as if it were real source data.
func TestSuppliedConfigRefusesInvalidArguments(t *testing.T) {
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			content := validConfig
			if operation == "diff" {
				content = suppliedConfigPair(t, content, content)
			}
			valid := suppliedConfigTokens(content)
			for name, tokens := range map[string][]string{
				"missing":           nil,
				"empty":             {"content", ""},
				"malformed base64":  {"content", "c2VjcmV0!!"},
				"masked history":    {"content", "<redacted>"},
				"duplicate content": append(append([]string{}, valid...), valid...),
				"path argument":     append(append([]string{}, valid...), "file", "/etc/passwd"),
				"write option":      append(append([]string{}, valid...), "-w"),
				"path label":        append(append([]string{}, valid...), "name", "../router.conf"),
			} {
				t.Run(name, func(t *testing.T) {
					got := suppliedConfigAnswer(t, operation, tokens)
					if got.Code == 0 || got.Payload != nil || got.Diagnostic == "" {
						t.Errorf("invalid supplied arguments were not refused: %+v", got)
					}
					if strings.Contains(got.Diagnostic, "c2VjcmV0!!") {
						t.Fatal("malformed-base64 refusal disclosed source content")
					}
				})
			}
		})
	}
}

// TestSuppliedConfigCanceledCallerGetsNoAnswer prevents a disconnected caller
// from beginning inspection through the supplied-input route.
func TestSuppliedConfigCanceledCallerGetsNoAnswer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			content := validConfig
			if operation == "diff" {
				content = suppliedConfigPair(t, content, content)
			}
			got := suppliedConfigAnswerContext(t, ctx, operation, suppliedConfigTokens(content))
			if got.Code == 0 || got.Payload != nil || !strings.Contains(got.Diagnostic, context.Canceled.Error()) {
				t.Errorf("canceled caller received an answer: %+v", got)
			}
		})
	}
}

// TestSuppliedConfigLabelDoesNotReadOrRewriteAFile puts a different config at
// the label in the daemon's working directory. Supplied content must determine
// the answer, and the unrelated file must survive unchanged.
func TestSuppliedConfigLabelDoesNotReadOrRewriteAFile(t *testing.T) {
	t.Chdir(t.TempDir())
	const label = "uploaded.conf"
	const original = "system { identity { host-name local-file-must-survive; } }\n"
	if err := os.WriteFile(label, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	local := answerOn(t, "show config dump", []string{writeTestConfig(t, validConfig)}, false)
	tokens := append(suppliedConfigTokens(validConfig), "name", label)
	got := suppliedConfigAnswer(t, "dump", tokens)
	if encodedAnswer(t, got) != encodedAnswer(t, local) {
		t.Errorf("diagnostic label selected file bytes instead of supplied bytes")
	}
	after, err := os.ReadFile(filepath.Clean(label))
	if err != nil || string(after) != original {
		t.Errorf("supplied inspection changed the label's file: %q, %v", after, err)
	}
}

// TestSuppliedConfigDescriptorsMatchModel derives the config owner's supplied
// inputs and checks their real argument constraints. A generic form can join the
// same metadata and schema without owning a second command or size inventory.
func TestSuppliedConfigDescriptorsMatchModel(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	definitions := configyang.PathToArgDefs(loader)
	seen := 0
	for _, entry := range command.ListLocal() {
		input := entry.Meta.SuppliedInput
		if input == nil || !strings.HasPrefix(entry.Path, "show config supplied ") {
			continue
		}
		seen++
		t.Run(entry.Path, func(t *testing.T) {
			handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
			if handler == nil || len(rest) != 0 {
				t.Fatal("supplied-input metadata has no exact owner")
			}
			var content, name *command.ArgDef
			for _, def := range definitions[entry.Path] {
				if def.Name() == input.ContentArg {
					content = &def
				}
				if input.NameArg != "" && def.Name() == input.NameArg {
					name = &def
				}
			}
			if content == nil || content.Kind() != command.ArgString || !content.Mandatory() {
				t.Fatal("the supplied content argument is not a required modeled string")
			}
			bounds := slices.Collect(content.Lengths())
			if len(bounds) != 1 || bounds[0].Min != 4 || bounds[0].Max != command.ContentEncodedMax {
				t.Fatalf("modeled packet bounds differ from the shared codec: %+v", bounds)
			}
			if len(input.Fields) == 0 {
				if name == nil || command.ValidateArgString("router.conf", name) != nil ||
					command.ValidateArgString("../router.conf", name) == nil {
					t.Fatal("single-input descriptor lost its modeled basename label")
				}
			} else {
				fields := []string{suppliedBefore, suppliedAfter}
				if entry.Path == "show config supplied completion" {
					fields = []string{"configuration", "input"}
				}
				if input.NameArg != "" || !slices.Equal(input.Fields, fields) {
					t.Fatalf("comparison descriptor differs from the actual envelope: %+v", input)
				}
			}
		})
	}
	if seen == 0 {
		t.Fatal("no registered command declares supplied input")
	}
}

// TestSuppliedConfigComparisonRetainsSideDiagnostics proves both native
// parsers run after the envelope has been accepted, with the failed side named.
func TestSuppliedConfigComparisonRetainsSideDiagnostics(t *testing.T) {
	for _, side := range []string{suppliedBefore, suppliedAfter} {
		t.Run(side, func(t *testing.T) {
			before, after := validConfig, validConfig
			if side == suppliedBefore {
				before = "not-native-configuration;"
			} else {
				after = "not-native-configuration;"
			}
			got := suppliedConfigAnswer(t, "diff", suppliedConfigTokens(suppliedConfigPair(t, before, after)))
			if got.Code == 0 || got.Payload != nil || !strings.Contains(got.Diagnostic, side+" configuration") {
				t.Errorf("comparison lost the failed source's diagnostic: %+v", got)
			}
		})
	}
}

// TestSuppliedConfigPipesPreserveOwnedAnswerShapes joins the actual input
// declarations to the shared pipe validator. File-based forms keep their native
// shapes, and completion declares its own document.
func TestSuppliedConfigPipesPreserveOwnedAnswerShapes(t *testing.T) {
	seen := 0
	for _, entry := range command.ListLocal() {
		if entry.Meta.SuppliedInput == nil || !strings.HasPrefix(entry.Path, "show config supplied ") {
			continue
		}
		seen++
		t.Run(entry.Path, func(t *testing.T) {
			operation := strings.TrimPrefix(entry.Path, "show config supplied ")
			shape, declared := command.ShapeForCommand(entry.Path)
			if entry.Path == "show config supplied completion" {
				if !declared || shape != command.ShapeDoc {
					t.Fatal("supplied completion does not declare its owned completion document")
				}
			} else {
				fileShape, fileDeclared := command.ShapeForCommand("show config " + operation)
				if !declared || !fileDeclared || shape != fileShape {
					t.Fatal("supplied and file forms do not declare the same answer shape")
				}
			}
			content := validConfig
			if entry.Path == "show config supplied completion" {
				data, err := json.Marshal(map[string]string{suppliedConfiguration: content, suppliedInput: "set router"})
				if err != nil {
					t.Fatal(err)
				}
				content = string(data)
			} else if len(entry.Meta.SuppliedInput.Fields) != 0 {
				content = suppliedConfigPair(t, content, content)
			}
			line := entry.Path + " content " + base64.StdEncoding.EncodeToString([]byte(content))
			clean, format, refusal := command.ProcessPipesDefaultFormatChecked(line+" | json compact", "")
			if refusal != "" || clean != line || format == nil {
				t.Fatalf("the shared JSON pipe refused supplied input: %s", refusal)
			}
			_, _, refusal = command.ProcessPipesDefaultFormatChecked(line+" | match needle", "")
			if !strings.Contains(refusal, "match cannot apply here") {
				t.Fatalf("a row pipe was not refused by its declared shape: %s", refusal)
			}
		})
	}
	if seen == 0 {
		t.Fatal("no supplied input reached the shared pipe contract")
	}
}
