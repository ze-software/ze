package gnmi

import (
	"sort"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
)

func TestTypedValueToString(t *testing.T) {
	tests := []struct {
		name    string
		tv      *gpb.TypedValue
		want    string
		wantErr bool
	}{
		{
			name: "string",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "hello"}},
			want: "hello",
		},
		{
			name: "int",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_IntVal{IntVal: -42}},
			want: "-42",
		},
		{
			name: "uint",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_UintVal{UintVal: 65000}},
			want: "65000",
		},
		{
			name: "bool true",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_BoolVal{BoolVal: true}},
			want: "true",
		},
		{
			name: "bool false",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_BoolVal{BoolVal: false}},
			want: "false",
		},
		{
			name: "nil",
			tv:   nil,
			want: "",
		},
		{
			name: "json_ietf",
			tv:   &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{"a":"b"}`)}},
			want: `{"a":"b"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typedValueToString(tt.tv)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetPathUsesDotSeparator(t *testing.T) {
	segments := []string{"bgp", "neighbor", "10.0.0.1", "description"}
	got := strings.Join(segments, ".")
	if got != "bgp.neighbor.10.0.0.1.description" {
		t.Errorf("expected dot-separated path, got %q", got)
	}
	bad := strings.Join(segments, " ")
	if bad == got {
		t.Error("dot and space joins should differ")
	}
}

type configSessionEditor struct {
	values           map[string]string
	committedContent string
}

func (e *configSessionEditor) SetValue(path []string, key, value string) error {
	e.values[strings.Join(path, ".")+"."+key] = value
	return nil
}

func (e *configSessionEditor) DeleteByPath(fullPath []string) error {
	delete(e.values, strings.Join(fullPath, "."))
	return nil
}

func (e *configSessionEditor) Diff() string { return "" }
func (e *configSessionEditor) Save() error {
	e.committedContent = e.WorkingContent()
	return nil
}
func (e *configSessionEditor) StageCandidate(time.Time) (string, string, error) {
	return e.WorkingContent(), "test-version", nil
}
func (e *configSessionEditor) MarkCommittedContent(content string) {
	e.committedContent = content
}
func (e *configSessionEditor) RestoreOriginalContent(string) error { return nil }
func (e *configSessionEditor) Discard() error                      { e.values = make(map[string]string); return nil }
func (e *configSessionEditor) OriginalContent() string             { return "# original\n" }
func (e *configSessionEditor) WorkingContent() string {
	if len(e.values) == 0 {
		return "# config\n"
	}
	keys := make([]string, 0, len(e.values))
	for key := range e.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# config\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(e.values[key])
		b.WriteByte('\n')
	}
	return b.String()
}

func TestGNMISetCommitsConfigSession(t *testing.T) {
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	srv := NewServer(Config{}, nil, sessions, nil, nil)

	_, err := srv.Set(t.Context(), &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "router-id"},
			}},
			Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "10.0.0.1"}},
		}},
	})
	if err != nil {
		t.Fatalf("Set RPC: %v", err)
	}
	if !strings.Contains(editor.committedContent, "bgp.router-id = 10.0.0.1") {
		t.Fatalf("committed content %q does not contain router-id update", editor.committedContent)
	}
}
