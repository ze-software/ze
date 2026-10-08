package yang

import (
	"strings"
	"testing"
)

func TestTrafficCmdSchemaOwnsTraffic(t *testing.T) {
	required := []string{
		`ze:command "ze-traffic:show-traffic"`,
	}
	for _, token := range required {
		if !strings.Contains(ZeTrafficCmdYANG, token) {
			t.Errorf("traffic command schema missing %q", token)
		}
	}
}
