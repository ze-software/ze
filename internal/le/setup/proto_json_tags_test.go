package setup

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestProtoJSONTagsIsAZeroArgumentNativeSetupAction(t *testing.T) {
	found := 0
	for _, action := range Actions().Actions {
		if action.Verb != protoJSONTagsVerb {
			continue
		}
		found++
		if !action.Writes {
			t.Errorf("proto-json-tags action is not marked as writing: %#v", action)
		}
	}
	if found != 1 {
		t.Fatalf("proto-json-tags action occurs %d times", found)
	}
}

func TestProtoJSONTagsRewritesEveryDeclaredOccurrenceAndRepeatsCleanly(t *testing.T) {
	proto := `
message One {
  string session_id = 1 [json_name = "session-id"];
}
message Two {
  string session_id = 1 [json_name = "session-id"];
  bool read_only = 2 [json_name = "read-only"];
}
`
	generated := `
SessionID string ` + "`" + `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"` + "`" + `
SessionID string ` + "`" + `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"` + "`" + `
ReadOnly bool ` + "`" + `protobuf:"varint,2,opt,name=read_only,json=readOnly,proto3" json:"read_only,omitempty"` + "`" + `
`

	rewritten, err := rewriteProtoJSONTags(proto, generated)
	if err != nil {
		t.Fatalf("rewriteProtoJSONTags: %v", err)
	}
	if count := strings.Count(rewritten, `json:"session-id,omitempty"`); count != 2 {
		t.Errorf("session-id tag count = %d", count)
	}
	if count := strings.Count(rewritten, `json:"read-only,omitempty"`); count != 1 {
		t.Errorf("read-only tag count = %d", count)
	}
	if strings.Contains(rewritten, `json:"session_id,omitempty"`) || strings.Contains(rewritten, `json:"read_only,omitempty"`) {
		t.Errorf("source tags remain:\n%s", rewritten)
	}

	repeated, err := rewriteProtoJSONTags(proto, rewritten)
	if err != nil {
		t.Fatalf("idempotent rewrite: %v", err)
	}
	if repeated != rewritten {
		t.Error("an already rewritten file changed")
	}
}

func TestProtoJSONTagsRefusesAmbiguousInputs(t *testing.T) {
	tests := []struct {
		name      string
		proto     string
		generated string
		want      string
	}{
		{
			name: "no explicit names", proto: "message One { string session_id = 1; }\n",
			generated: "package proto\n", want: "no explicit json_name options",
		},
		{
			name:      "tag count mismatch",
			proto:     "message One {\n string session_id = 1 [json_name = \"session-id\"];\n}\n",
			generated: "package proto\n", want: "generated Go has 0",
		},
		{
			name: "conflicting names",
			proto: `
message One {
  string session_id = 1 [json_name = "session-id"];
}
message Two {
  string session_id = 1 [json_name = "other-id"];
}
`,
			generated: "package proto\n", want: "conflicting json_name options",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := rewriteProtoJSONTags(test.proto, test.generated)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestProtoJSONTagsRunWritesOnlyAChangedGeneratedFile(t *testing.T) {
	files := map[string][]byte{
		"schema.proto": []byte("message One {\n string session_id = 1 [json_name = \"session-id\"];\n}\n"),
		"schema.pb.go": []byte("SessionID string `json:\"session_id,omitempty\"`\n"),
	}
	writes := 0
	rewriter := &protoJSONTags{
		ProtoPath: "schema.proto", GeneratedPath: "schema.pb.go",
		ReadFile: func(path string) ([]byte, error) {
			contents, found := files[path]
			if !found {
				return nil, os.ErrNotExist
			}
			return append([]byte(nil), contents...), nil
		},
		WriteFile: func(path string, contents []byte, mode os.FileMode) error {
			writes++
			if path != "schema.pb.go" || mode != 0o644 {
				t.Fatalf("write = %s mode %#o", path, mode)
			}
			files[path] = append([]byte(nil), contents...)
			return nil
		},
	}
	changed, err := rewriter.run()
	if err != nil || !changed {
		t.Fatalf("first Run() = changed %t, error %v", changed, err)
	}
	changed, err = rewriter.run()
	if err != nil || changed {
		t.Fatalf("second Run() = changed %t, error %v", changed, err)
	}
	if writes != 1 {
		t.Fatalf("write count = %d", writes)
	}
}

func TestProtoJSONTagsRunReportsReadAndWriteFailures(t *testing.T) {
	readFailure := &protoJSONTags{
		ProtoPath: "schema.proto", GeneratedPath: "schema.pb.go",
		ReadFile: func(string) ([]byte, error) { return nil, errors.New("disk unavailable") },
	}
	if _, err := readFailure.run(); err == nil || !strings.Contains(err.Error(), "read proto source schema.proto: disk unavailable") {
		t.Fatalf("read error = %v", err)
	}

	writeFailure := &protoJSONTags{
		ProtoPath: "schema.proto", GeneratedPath: "schema.pb.go",
		ReadFile: func(path string) ([]byte, error) {
			if path == "schema.proto" {
				return []byte("message One {\n string session_id = 1 [json_name = \"session-id\"];\n}\n"), nil
			}
			return []byte("SessionID string `json:\"session_id,omitempty\"`\n"), nil
		},
		WriteFile: func(string, []byte, os.FileMode) error { return errors.New("read-only filesystem") },
	}
	if _, err := writeFailure.run(); err == nil || !strings.Contains(err.Error(), "write generated Go schema.pb.go: read-only filesystem") {
		t.Fatalf("write error = %v", err)
	}
}
