package config

import (
	"strings"
	"testing"
	"time"
)

// TestValidateArchiveLocation_Valid verifies the three schemes the uploader
// dispatches are accepted.
//
// VALIDATES: file://, http:// and https:// locations pass.
// PREVENTS: a valid archive destination refused at commit.
func TestValidateArchiveLocation_Valid(t *testing.T) {
	for _, location := range []string{
		"file:///backups/configs",
		"http://config-server.example.com/archive",
		"https://config-server.example.com/archive",
	} {
		if err := ValidateArchiveLocation(location); err != nil {
			t.Errorf("ValidateArchiveLocation(%q) = %v, want nil", location, err)
		}
	}
}

// TestValidateArchiveLocation_Invalid verifies every other form is refused
// with a message that names the value and the accepted forms.
//
// VALIDATES: scp, sftp and ftp schemes, an empty value and a value with no
// scheme are refused, and each message carries the accepted forms.
// PREVENTS: a location the uploader cannot dispatch committing clean.
func TestValidateArchiveLocation_Invalid(t *testing.T) {
	for _, location := range []string{
		"scp://user@host:/path",
		"sftp://user@host/path",
		"ftp://server/path",
		"",
		"just-a-path",
		"://bad",
	} {
		err := ValidateArchiveLocation(location)
		if err == nil {
			t.Errorf("ValidateArchiveLocation(%q) = nil, want an error", location)
			continue
		}
		if !strings.Contains(err.Error(), "file://, http:// or https://") {
			t.Errorf("ValidateArchiveLocation(%q) = %q, want the accepted forms named", location, err)
		}
	}
}

// TestValidateArchiveLocation_RedactsCredentials verifies a refusal never
// carries the password of the URL it refuses.
//
// VALIDATES: the unsupported-scheme and missing-scheme refusals drop userinfo.
// PREVENTS: a password in a config error, a log line or a commit refusal.
func TestValidateArchiveLocation_RedactsCredentials(t *testing.T) {
	for _, location := range []string{
		"ftp://admin:s3cret@host.example.com/path",
		"//admin:s3cret@host/path",
	} {
		err := ValidateArchiveLocation(location)
		if err == nil {
			t.Fatalf("ValidateArchiveLocation(%q) = nil, want an error", location)
		}
		if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("ValidateArchiveLocation(%q) leaks the password: %q", location, err)
		}
	}
}

// TestParseArchiveTimeout verifies a positive Go duration parses and every
// other value is an error naming the value.
//
// VALIDATES: 10s and 2m parse; "5 minutes", "0s", "-1s" and "" are refused.
// PREVENTS: an invalid timeout replaced by 30s in silence.
func TestParseArchiveTimeout(t *testing.T) {
	valid := map[string]time.Duration{"10s": 10 * time.Second, "2m": 2 * time.Minute}
	for value, want := range valid {
		got, err := ParseArchiveTimeout(value)
		if err != nil {
			t.Errorf("ParseArchiveTimeout(%q) error %v, want %v", value, err, want)
			continue
		}
		if got != want {
			t.Errorf("ParseArchiveTimeout(%q) = %v, want %v", value, got, want)
		}
	}
	for _, value := range []string{"5 minutes", "0s", "-1s", ""} {
		_, err := ParseArchiveTimeout(value)
		if err == nil {
			t.Errorf("ParseArchiveTimeout(%q) = nil error, want a refusal", value)
			continue
		}
		if !strings.Contains(err.Error(), "30s or 2m") {
			t.Errorf("ParseArchiveTimeout(%q) = %q, want the accepted form named", value, err)
		}
	}
}
