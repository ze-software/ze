// Design: docs/architecture/config/environment-block.md -- environment/log/* is owned by slogutil.ApplyLogConfig
// Related: internal/core/slogutil/slogutil.go -- validBackends, the one Go declaration of the backends
//
// Goal: prove the backends an operator can write at environment/log/backend are
// backends slogutil writes to, so a word cannot exist on one side alone. The
// test sits here rather than beside validBackends because slogutil is under
// the model and MUST NOT import the loader (./le arch tier check); this package
// already imports both. Method: read the enumeration out of the loaded model
// with configyang.EnumValues, which fails on a leaf that declares no
// enumeration, and hold it to slogutil.BackendNames in both directions.

package config

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/slogutil"

	// The blank import registers ze-hub-conf with the loader, which declares
	// the leaf read below.
	_ "github.com/ze-software/ze/internal/component/hub/yang"
)

const logBackendLeaf = "environment/log/backend"

// logBackendEnvOnly is the one backend slogutil writes to that the leaf does
// not offer. The appliance sets it through ze.log.backend=kmsg,stderr
// (docs/guide/appliance.md), a comma-separated pair an enumeration leaf cannot
// hold, and no page offers `backend kmsg` in a config file. It is named here
// so a fifth backend added to either side alone still turns this test red;
// offering kmsg in the leaf is a config decision, and taking it deletes this
// exclusion.
const logBackendEnvOnly = "kmsg"

// TestLogBackendLeafMatchesSlogutil holds the leaf to slogutil's backend set.
func TestLogBackendLeafMatchesSlogutil(t *testing.T) {
	model, err := configyang.EnumValues(logBackendLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", logBackendLeaf, err)
	}

	written := slogutil.BackendNames()
	if !slices.Contains(written, logBackendEnvOnly) {
		t.Fatalf("slogutil no longer writes to %q, so the exclusion above excuses nothing: delete it", logBackendEnvOnly)
	}
	offered := slices.DeleteFunc(slices.Clone(written), func(name string) bool { return name == logBackendEnvOnly })

	if !slices.Equal(model, offered) {
		t.Errorf("the log backends disagree: the model at %s holds %v and slogutil writes to %v (%s excluded, env-only). "+
			"A word only the model carries is refused by ApplyLogConfig with a warning and the level stays where it was, "+
			"and a word only Go carries is one no config file can ask for",
			logBackendLeaf, model, offered, logBackendEnvOnly)
	}
}
