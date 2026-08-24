//go:build !ze_ssh

package infra_test

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/config/infra"
)

// TestExtractAuthUsersAvailableWithoutSSHFeature is a compile and link seam:
// shared credential extraction must stay callable when the gated SSH package is
// absent. Parser behavior is covered by the hub build-composition tests.
func TestExtractAuthUsersAvailableWithoutSSHFeature(t *testing.T) {
	users := infra.ExtractAuthUsers(map[string]any{
		"authentication": map[string]any{
			"user": map[string]any{
				"operator": map[string]any{
					"password": "bcrypt-hash",
					"profile":  []string{"readonly"},
				},
			},
		},
	})
	if len(users) != 1 {
		t.Fatalf("ExtractAuthUsers returned %d users, want 1", len(users))
	}
	if users[0].Name != "operator" || users[0].Hash != "bcrypt-hash" {
		t.Fatalf("shared user extraction = %#v, want operator with bcrypt-hash", users[0])
	}
	if len(users[0].Profiles) != 1 || users[0].Profiles[0] != "readonly" {
		t.Fatalf("shared user profiles = %v, want [readonly]", users[0].Profiles)
	}
}

func TestSSHExtractedConfigIsTransportOnly(t *testing.T) {
	if field, ok := reflect.TypeFor[infra.SSHExtractedConfig]().FieldByName("Users"); ok {
		t.Fatalf("SSHExtractedConfig still carries shared identity through field %s", field.Name)
	}
}
