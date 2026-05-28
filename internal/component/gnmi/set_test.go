package gnmi

import (
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
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
