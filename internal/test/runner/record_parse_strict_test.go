// VALIDATES: every generic .ci directive family refuses a key its parser does
//            not read, a segment with no `=`, and a key given twice, and the
//            refusal names the line, the directive, the key and the keys it
//            accepts.
// PREVENTS: the silent-drop class the 2026-10-10 parseOption journal row
//           records. Only option=timeout, needs-linux and needs-arch checked
//           their keys; every other option, every action= arm, cmd=api,
//           await=stderr and http= dropped an unknown key, so an assertion or
//           a setting the author wrote did nothing and the test still passed.

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/ci"
)

func TestParseCIEveryFamilyRefusesUnknownKey(t *testing.T) {
	tests := []struct {
		line   string
		wantIn []string
	}{
		{"option=file:path=a.conf:mode=x", []string{"option=file", `"mode"`, "accepts path"}},
		{"option=asn:value=65000:bogus=1", []string{"option=asn", `"bogus"`, "accepts value"}},
		{"option=bind:value=ipv6:bogus=1", []string{"option=bind", `"bogus"`}},
		{"option=tcp_connections:value=2:bogus=1", []string{"option=tcp_connections", `"bogus"`}},
		{"option=open:value=x:bogus=1", []string{"option=open", `"bogus"`}},
		{"option=update:value=x:bogus=1", []string{"option=update", `"bogus"`}},
		{"option=env:var=X:value=1:bogus=1", []string{"option=env", `"bogus"`, "accepts value, var"}},
		{"option=skip-os:value=plan9:bogus=1", []string{"option=skip-os", `"bogus"`}},
		{"option=needs-path:value=x:bogus=1", []string{"option=needs-path", `"bogus"`, "accepts hint, value"}},
		{"option=netns-link:name=d0:bogus=1", []string{"option=netns-link", `"bogus"`, "accepts address, name, peer, vlan"}},
		{"option=exclusive:group=g:bogus=1", []string{"option=exclusive", `"bogus"`, "accepts group"}},
		{"option=skip-env:var=X:bogus=1", []string{"option=skip-env", `"bogus"`}},
		{"action=notification:conn=1:seq=1:bogus=1:text=x", []string{"action=notification", `"bogus"`, "accepts conn, seq, text"}},
		{"action=send:conn=1:seq=1:bogus=1:hex=00", []string{"action=send", `"bogus"`}},
		{"action=rewrite:conn=1:seq=1:source=a:dest=b:bogus=1", []string{"action=rewrite", `"bogus"`}},
		{"action=sighup:conn=1:seq=1:bogus=1", []string{"action=sighup", `"bogus"`, "accepts conn, seq"}},
		{"action=sigterm:conn=1:seq=1:bogus=1", []string{"action=sigterm", `"bogus"`}},
		{"cmd=api:conn=1:seq=1:bogus=1:text=x", []string{"cmd=api", `"bogus"`, "accepts conn, seq, text"}},
		{"await=stderr:contains=x:bogus=1", []string{"await=stderr", `"bogus"`, "accepts contains, then, timeout"}},
		{"http=get:seq=1:url=http://h/:status=200:bogus=1", []string{"http=get", `"bogus"`}},
	}
	// text= and hex= consume the rest of the line, colons included
	// (ci.ParseKVPairs), so each unknown key is written before them.
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, err := parseCILines(t, tt.line)
			require.Error(t, err, "an unknown key must fail the file, not be dropped")
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
			assert.Contains(t, err.Error(), "line 1:")
		})
	}
}

func TestParseCIRefusesMalformedSegments(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantIn []string
	}{
		{"value with no key", "option=asn:65000", []string{"option=asn", `"65000"`, "accepts value"}},
		{"duplicate option key", "option=timeout:value=1s:value=9s", []string{"option=timeout", `"value"`, "twice"}},
		{"duplicate action key", "action=send:conn=1:seq=1:seq=2:hex=00", []string{"action=send", `"seq"`, "twice"}},
		{"duplicate http key", "http=get:seq=1:url=http://h/:status=200:status=404", []string{"http=get", `"status"`, "twice"}},
		{"non-letter key after a key", "expect=file:path=out.txt:!contains=x", []string{"expect=file", `"!contains"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCILines(t, tt.line)
			require.Error(t, err)
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
			assert.Contains(t, err.Error(), "line 1:")
		})
	}
}

// TestParseCIStrictKeysStillParse is the accepting polarity: one line per
// family carrying every key its arm reads.
func TestParseCIStrictKeysStillParse(t *testing.T) {
	rec, err := parseCILines(t,
		"option=env:var=ZE_X:value=1",
		"option=exclusive:group=g",
		"option=netns-link:name=d0:peer=d1:address=10.0.0.1/24:vlan=10",
		"action=rewrite:conn=1:seq=1:source=a:dest=b",
		"cmd=api:conn=1:seq=1:text=hello",
		"await=stderr:contains=ready:timeout=5s:then=stop",
		"cmd=foreground:seq=1:exec=ze version",
		"http=get:seq=1:url=http://h:8080/x:status=200:contains=ok:header=Referer: http://h:8080/:timeout=2s",
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"ZE_X=1"}, rec.EnvVars)
	assert.Equal(t, "g", rec.ExclusiveGroup)
	require.Len(t, rec.HTTPChecks, 1)
	assert.Equal(t, "http://h:8080/x", rec.HTTPChecks[0].URL)
	assert.Equal(t, []string{"ready"}, rec.AwaitStderr)
}

// TestParseCIChecksTypeBeforeSegments proves parseLine looks the directive type
// up before it splits the key=value tail.
//
// VALIDATES: an unknown type followed by a malformed segment is refused as an
// unknown type, naming the accepted types.
// PREVENTS: `option=bogus:65000` reporting the key-less segment with no accepted
// list, which sends the author to fix a segment of a directive that does not exist.
func TestParseCIChecksTypeBeforeSegments(t *testing.T) {
	tests := []struct {
		line   string
		wantIn []string
	}{
		{"option=bogus:65000", []string{`unknown option type "bogus"`, "accepts", "asn"}},
		{"action=sighub:65000", []string{`unknown action type "sighub"`, "accepts", "sighup"}},
		{"await=stdout:65000", []string{`unknown await type "stdout"`, "accepts", "stderr"}},
		{"expect=stdoutt:65000", []string{`unknown expect type "stdoutt"`, "accepts", "stdout"}},
		{"reject=stdoutt:65000", []string{`unknown reject type "stdoutt"`, "accepts", "stdout"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, err := parseCILines(t, tt.line)
			require.Error(t, err)
			assert.NotErrorIs(t, err, ci.ErrSegmentWithoutKey, "the type is checked before the segments")
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
			assert.Contains(t, err.Error(), "line 1:")
		})
	}
}
