package registry

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestUnloadedDependencyErrorNamesWhatToConfigure verifies the refusal for a
// dependency a plugin declared at Stage 1 that nothing configured or loaded.
//
// VALIDATES: spec-operator-errors-name-the-fix AC-10. A dependency this build
// carries names the internal block to add; a name no build carries names the
// external block to add; a dependency this build was compiled without names
// its ze_ build tags; every case keeps the other fix, removing the
// configuration that loads the plugin, and keeps errors.Is
// ErrMissingDependency and errors.As *opererr.Error.
// PREVENTS: "missing dependency: plugin "x" requires "y"", which named neither
// why y is absent nor what to write, so a compiled-out feature and a plugin the
// operator only forgot to configure read alike.
// METHOD: an empty registry holds "provider" only; build the refusal for it,
// for an unknown name, and for the real gated name bgp-bmp, and compare the
// whole rendered text.
func TestUnloadedDependencyErrorNamesWhatToConfigure(t *testing.T) {
	snap := Snapshot()
	Reset()
	t.Cleanup(func() { Restore(snap) })

	if err := Register(validReg("provider")); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		dependency string
		tags       []string
		want       string
	}{
		{
			name:       "dependency in this build names the internal block",
			dependency: "provider",
			want: `plugin "feed" needs plugin "provider", and the configuration does not load it` + "\n" +
				`"feed" does not work without "provider", so ze refuses to start it` + "\n" +
				`fix: add 'plugin { internal provider { use provider; } }' to the config, ` +
				`or remove the configuration that loads "feed"`,
		},
		{
			name:       "name no build carries names the external block",
			dependency: "collector",
			want: `plugin "feed" needs plugin "collector", and no plugin called "collector" is configured` + "\n" +
				`"feed" does not work without "collector", so ze refuses to start it; ` +
				`ze does not include a plugin of that name, so only an external program can provide it` + "\n" +
				`fix: add 'plugin { external collector { run "<the command that starts collector>"; } }' to the config, ` +
				`or remove the configuration that loads "feed"`,
		},
		{
			name:       "compiled-out dependency names its build tags",
			dependency: "bgp-bmp",
			tags:       []string{"ze_bgp", "ze_bmp"},
			want: `plugin "feed" needs plugin "bgp-bmp", and this build of ze was compiled without it` + "\n" +
				`"feed" does not work without "bgp-bmp", so ze refuses to start it` + "\n" +
				`fix: rebuild ze with ze_bgp ze_bmp added to its -tags list, ` +
				`or remove the configuration that loads "feed"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error = UnloadedDependencyError("feed", tc.dependency)
			if !errors.Is(err, ErrMissingDependency) {
				t.Fatalf("errors.Is(err, ErrMissingDependency) = false, err = %v", err)
			}
			var missing *MissingDependencyError
			if !errors.As(err, &missing) {
				t.Fatalf("error is not a *MissingDependencyError: %T %v", err, err)
			}
			if !slices.Equal(missing.Tags, tc.tags) {
				t.Errorf("Tags = %v, want %v", missing.Tags, tc.tags)
			}
			if got := err.Error(); got != tc.want {
				t.Errorf("message:\n got %q\nwant %q", got, tc.want)
			}
			var operator *opererr.Error
			if !errors.As(err, &operator) {
				t.Fatalf("errors.As(err, *opererr.Error) = false, err = %v", err)
			}
			if operator.Error() != tc.want {
				t.Errorf("operator error:\n got %q\nwant %q", operator.Error(), tc.want)
			}
		})
	}
}

// TestMissingDependencyErrorZeroValueNamesTheDefect verifies a value built
// without a constructor renders a refusal that names the defect, never an
// empty or misleading fix.
//
// VALIDATES: the absenceUnspecified branch of MissingDependencyError.refusal.
// PREVENTS: a composite literal rendering as one of the real absences, or
// opererr.New panicking on an empty part.
// METHOD: render a literal that sets only the names.
func TestMissingDependencyErrorZeroValueNamesTheDefect(t *testing.T) {
	err := &MissingDependencyError{Plugin: "feed", Dependency: "provider"}
	want := `plugin "feed" needs plugin "provider", and ze did not record why it is missing` + "\n" +
		`"feed" does not work without "provider", so ze refuses to start it` + "\n" +
		`fix: report this message to the ze developers: the refusal was built without its constructor`
	if got := err.Error(); got != want {
		t.Errorf("message:\n got %q\nwant %q", got, want)
	}
}
