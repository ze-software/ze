package gnmi

import (
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestPathTranslation(t *testing.T) {
	tests := []struct {
		name    string
		path    *gpb.Path
		want    []string
		wantErr bool
	}{
		{
			name: "simple container path",
			path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "global"},
			}},
			want: []string{"bgp", "global"},
		},
		{
			name: "single leaf",
			path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "hostname"},
			}},
			want: []string{"hostname"},
		},
		{
			name:    "nil path",
			path:    nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty path element name",
			path:    &gpb.Path{Elem: []*gpb.PathElem{{Name: ""}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pathToSegments(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pathToSegments() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !sliceEqual(got, tt.want) {
				t.Errorf("pathToSegments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathWithListKeys(t *testing.T) {
	tests := []struct {
		name    string
		path    *gpb.Path
		want    []string
		wantErr bool
	}{
		{
			name: "list with single key",
			path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "neighbor", Key: map[string]string{"address": "10.0.0.1"}},
				{Name: "description"},
			}},
			want: []string{"bgp", "neighbor", "10.0.0.1", "description"},
		},
		{
			name: "multiple list keys rejected",
			path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "routing"},
				{Name: "entry", Key: map[string]string{"vrf": "default", "prefix": "10.0.0.0/8"}},
			}},
			wantErr: true,
		},
		{
			name: "list without key",
			path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "neighbor"},
			}},
			want: []string{"bgp", "neighbor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pathToSegments(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pathToSegments() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !sliceEqual(got, tt.want) {
				t.Errorf("pathToSegments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathDepthBoundary(t *testing.T) {
	elems := make([]*gpb.PathElem, maxPathDepth)
	for i := range elems {
		elems[i] = &gpb.PathElem{Name: "a"}
	}
	if _, err := pathToSegments(&gpb.Path{Elem: elems}); err != nil {
		t.Fatalf("maxPathDepth should be accepted, got: %v", err)
	}

	elems = append(elems, &gpb.PathElem{Name: "overflow"})
	if _, err := pathToSegments(&gpb.Path{Elem: elems}); err == nil {
		t.Fatal("maxPathDepth+1 should be rejected")
	}
}

func TestSegmentsToPath(t *testing.T) {
	listNames := map[string]bool{"neighbor": true}

	tests := []struct {
		name      string
		segments  []string
		wantElems int
	}{
		{
			name:      "simple",
			segments:  []string{"bgp", "global"},
			wantElems: 2,
		},
		{
			name:      "with list key",
			segments:  []string{"bgp", "neighbor", "10.0.0.1", "description"},
			wantElems: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := segmentsToPath(tt.segments, listNames)
			if len(p.Elem) != tt.wantElems {
				t.Errorf("segmentsToPath() elem count = %d, want %d", len(p.Elem), tt.wantElems)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
