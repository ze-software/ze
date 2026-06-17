package provision

import (
	"slices"
	"testing"
)

func TestWithInstallLogDefaults(t *testing.T) {
	tests := []struct {
		name     string
		environ  []string
		wantAdd  []string // keys (assignments) expected to be appended
		wantSame bool     // true => returned slice must equal input (no change)
	}{
		{
			name:    "no log set adds all three subsystems",
			environ: []string{"PATH=/bin", "HOME=/root"},
			wantAdd: []string{"ze.log.dhcpserver=info", "ze.log.tftpserver=info", "ze.log.imageserver=info"},
		},
		{
			name:     "explicit global ze.log is respected",
			environ:  []string{"PATH=/bin", "ze.log=debug"},
			wantSame: true,
		},
		{
			name:     "global ZE_LOG (uppercase underscore form) is respected",
			environ:  []string{"PATH=/bin", "ZE_LOG=warn"},
			wantSame: true,
		},
		{
			name:    "one subsystem already set: only the others are added",
			environ: []string{"ze.log.dhcpserver=debug"},
			wantAdd: []string{"ze.log.tftpserver=info", "ze.log.imageserver=info"},
		},
		{
			name:    "underscore form of a subsystem is recognized",
			environ: []string{"ze_log_imageserver=debug"},
			wantAdd: []string{"ze.log.dhcpserver=info", "ze.log.tftpserver=info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withInstallLogDefaults(tt.environ)

			if tt.wantSame {
				if !slices.Equal(got, tt.environ) {
					t.Fatalf("expected unchanged environ, got %v", got)
				}
				return
			}

			// Original entries must be preserved.
			for _, e := range tt.environ {
				if !slices.Contains(got, e) {
					t.Errorf("original entry %q dropped", e)
				}
			}
			// Expected additions must be present.
			for _, want := range tt.wantAdd {
				if !slices.Contains(got, want) {
					t.Errorf("expected %q to be added, got %v", want, got)
				}
			}
			// No subsystem we did not intend to add should appear.
			added := got[len(tt.environ):]
			if len(added) != len(tt.wantAdd) {
				t.Errorf("added %v, want exactly %v", added, tt.wantAdd)
			}
		})
	}
}

func TestHasEnvKeyNormalization(t *testing.T) {
	env := []string{"ZE_LOG=info", "PATH=/bin"}
	for _, form := range []string{"ze.log", "ze_log", "ZE.LOG", "ZE_LOG"} {
		if !hasEnvKey(env, form) {
			t.Errorf("hasEnvKey did not match %q against ZE_LOG", form)
		}
	}
	if hasEnvKey(env, "ze.log.dhcpserver") {
		t.Error("ze.log.dhcpserver must not match the global ze.log entry")
	}
}
