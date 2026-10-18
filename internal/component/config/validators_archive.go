// Design: docs/guide/config-archive.md — archive location and timeout forms
// Related: validators_register.go — registers archive-location and archive-timeout
// Related: archive/archive.go — the uploader that dispatches on these schemes and uses the timeout

package config

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/redact"
)

// Archive URL schemes. They are declared here, not in the archive package,
// because config validates them at commit and the archive package imports
// config: the uploader's dispatch reads these same constants.
const (
	ArchiveSchemeFile  = "file"
	ArchiveSchemeHTTP  = "http"
	ArchiveSchemeHTTPS = "https"
)

// archiveLocationForms is the accepted-forms clause every location refusal ends with.
const archiveLocationForms = "write a file://, http:// or https:// URL, such as file:///var/backups/ze or https://archive.example.com/configs"

var errEmptyArchiveLocation = errors.New("archive location is empty; " + archiveLocationForms)

// ValidateArchiveLocation answers whether the archive uploader can write to
// this URL. It is the one check behind both the commit-time `archive-location`
// validator and the manual `ze config archive` command, so a location that
// commits is a location the uploader dispatches.
func ValidateArchiveLocation(rawURL string) error {
	if rawURL == "" {
		return errEmptyArchiveLocation
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("archive location %q is not a URL (%w); %s", redact.URL(rawURL), redact.URLError(err), archiveLocationForms)
	}

	switch parsed.Scheme {
	case ArchiveSchemeFile, ArchiveSchemeHTTP, ArchiveSchemeHTTPS:
		return nil
	case "":
		return fmt.Errorf("archive location %q has no URL scheme, so Ze cannot tell where to write; %s",
			redact.URL(rawURL), archiveLocationForms)
	}

	return fmt.Errorf("archive location %q uses scheme %q, which Ze cannot upload to; %s",
		redact.URL(rawURL), parsed.Scheme, archiveLocationForms)
}

// ParseArchiveTimeout converts an archive `timeout` leaf to a duration. A
// value that is not a Go duration, or is not above zero, is an error: an
// invalid timeout never becomes the 30s default, which is for an absent leaf.
func ParseArchiveTimeout(value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("archive timeout %q is not a Go duration; write a number and a unit, such as 30s or 2m", value)
	}
	if d <= 0 {
		return 0, fmt.Errorf("archive timeout %q is not above zero, so every upload would give up at once; write a positive duration, such as 30s or 2m", value)
	}
	return d, nil
}

// ArchiveLocationValidator refuses an archive location the uploader cannot
// dispatch. Without it the manual command was the only check, and a
// `trigger commit` entry with an ftp:// location committed and then failed
// every later commit's archive at runtime.
func ArchiveLocationValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(_ string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("archive location must be a string, got %T", value)
			}
			return ValidateArchiveLocation(str)
		},
	}
}

// ArchiveTimeoutValidator refuses an archive timeout that is not a positive
// Go duration, which the uploader used to replace with 30s in silence.
func ArchiveTimeoutValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(_ string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("archive timeout must be a string, got %T", value)
			}
			_, err := ParseArchiveTimeout(str)
			return err
		},
	}
}
