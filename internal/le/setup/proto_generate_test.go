package setup

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProtoGenerateActionIsRegisteredAsWriting(t *testing.T) {
	found := 0
	for _, action := range Actions().Actions {
		if action.Verb != protoGenerateVerb {
			continue
		}
		found++
		if !action.Writes {
			t.Errorf("proto-generate action is not marked as writing: %#v", action)
		}
	}
	if found != 1 {
		t.Fatalf("proto-generate action occurs %d times", found)
	}
}

func TestProtoGeneratorRunsPinnedNativePipeline(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/ze\n\ngo 1.26\n")
	write(protoSourceRel, "syntax = \"proto3\";\nmessage Request {\n  string request_id = 1 [json_name = \"requestID\"];\n}\n")

	var calls []Cmd
	shell := &Shell{
		Look: func(name string) (string, bool) { return "/usr/bin/" + name, name == "protoc" },
		Exec: func(_ context.Context, command Cmd) Result {
			calls = append(calls, command)
			if command.Argv[0] == "protoc" {
				write(protoGeneratedRel, "type Request struct { RequestId string `json:\"request_id,omitempty\"` }\n")
				write(grpcGeneratedRel, "package proto\n")
			}
			return Result{Argv: command.Argv}
		},
	}
	report, err := (&protoGenerator{Root: root, Shell: shell}).run()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if want := []string{protoGeneratedRel, grpcGeneratedRel}; !reflect.DeepEqual(report.Files, want) {
		t.Fatalf("files = %v, want %v", report.Files, want)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %d, want two plugin builds and protoc", len(calls))
	}
	if got := strings.Join(calls[2].Argv, " "); got != "protoc --go_out=. --go_opt=module=example.test/ze --go-grpc_out=. --go-grpc_opt=module=example.test/ze api/proto/ze.proto" {
		t.Fatalf("protoc argv = %q", got)
	}
	generated, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(protoGeneratedRel)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `json:"requestID,omitempty"`) {
		t.Fatalf("explicit json_name was not applied: %s", generated)
	}
	for _, call := range calls {
		if call.Dir != root {
			t.Errorf("command dir = %q, want %q", call.Dir, root)
		}
	}
	if !environmentHolds(calls[2].Env, "PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)) {
		t.Errorf("protoc PATH does not start with the generated plugin directory")
	}
}

func TestProtoGeneratorRefusesMissingProtoc(t *testing.T) {
	_, err := (&protoGenerator{
		Root:  t.TempDir(),
		Shell: &Shell{Look: func(string) (string, bool) { return "", false }},
	}).run()
	if err == nil || !strings.Contains(err.Error(), "protoc not found") {
		t.Fatalf("missing protoc error = %v", err)
	}
}

func environmentHolds(environment []string, key, prefix string) bool {
	for _, item := range environment {
		if strings.HasPrefix(item, key+"="+prefix) {
			return true
		}
	}
	return false
}
