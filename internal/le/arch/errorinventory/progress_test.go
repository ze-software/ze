package archerrorinventory

import "testing"

// TestDeclaredOTAProgressIsNotAnError bounds the non-error declarations to
// the two reviewed fixed progress messages. Changing the site, call shape or
// operands still yields a plain error row that the commit gate refuses.
func TestDeclaredOTAProgressIsNotAnError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     string
		fn       string
		call     string
		progress bool
	}{
		{"boot image", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, "streaming boot image...")`, true},
		{"boot code", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, "streaming boot code...")`, true},
		{"changed message", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, "streaming boot image failed")`, false},
		{"other function", "internal/appliance/cmd_push.go", "failedPush", `fmt.Fprintln(os.Stderr, "streaming boot image...")`, false},
		{"other file", "internal/appliance/other.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, "streaming boot image...")`, false},
		{"other writer", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprint(os.Stderr, "streaming boot image...")`, false},
		{"extra error", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, "streaming boot image...", err)`, false},
		{"dynamic message", "internal/appliance/cmd_push.go", "doPushUpdater", `fmt.Fprintln(os.Stderr, message)`, false},
		{"error writer", "internal/appliance/cmd_push.go", "doPushUpdater", `http.Error(nil, "streaming boot image...", 500)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package appliance\nimport (\"fmt\"; \"os\"; \"net/http\")\nfunc " + tc.fn + "(err error, message string) {\n" + tc.call + "\n}\n"
			rows := scanSources(t, map[string]string{tc.file: source})
			if tc.progress {
				if len(rows) != 0 {
					t.Fatalf("positive OTA progress was inventoried as an error: %+v", rows)
				}
				return
			}
			if len(rows) != 1 {
				t.Fatalf("unclassified error has %d rows, want 1: %+v", len(rows), rows)
			}
			if rows[0].Status != StatusPlain {
				t.Fatalf("unclassified error status = %s, want plain", rows[0].Status)
			}
			if !refused(&rows[0]) {
				t.Fatal("commit gate admitted an unclassified error")
			}
		})
	}
}
