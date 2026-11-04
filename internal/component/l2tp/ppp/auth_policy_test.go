package ppp

import (
	"slices"
	"testing"
)

// TestResolveAuthAllow proves defaults are least-permissive and explicit sets
// preserve user ordering while rejecting missing configured methods and none.
func TestResolveAuthAllow(t *testing.T) {
	for _, tc := range []struct {
		method  AuthMethod
		names   []string
		want    []AuthMethod
		invalid bool
	}{
		{AuthMethodCHAPMD5, nil, []AuthMethod{AuthMethodCHAPMD5}, false},
		{AuthMethodMSCHAPv2, nil, []AuthMethod{AuthMethodMSCHAPv2}, false},
		{AuthMethodPAP, nil, []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}, false},
		{AuthMethodNone, nil, nil, false},
		{AuthMethodPAP, []string{"pap", "ms-chap-v2", "chap-md5", "pap"}, []AuthMethod{AuthMethodPAP, AuthMethodMSCHAPv2, AuthMethodCHAPMD5}, false},
		{AuthMethodCHAPMD5, []string{"pap"}, nil, true},
		{AuthMethodCHAPMD5, []string{"none"}, nil, true},
		{AuthMethodNone, []string{"chap-md5"}, nil, true},
		{AuthMethodCHAPMD5, []string{"bad"}, nil, true},
		{AuthMethod(255), nil, nil, true},
	} {
		got, err := ResolveAuthAllow(tc.method, tc.names)
		if (err != nil) != tc.invalid {
			t.Fatalf("method %v names %v: error %v, invalid %v", tc.method, tc.names, err, tc.invalid)
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("method %v names %v: got %v, want %v", tc.method, tc.names, got, tc.want)
		}
	}
}
