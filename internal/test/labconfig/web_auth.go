// Design: docs/architecture/testing/runner-architecture.md -- shared browser authorization fixture.

package labconfig

import (
	"context"
	"fmt"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const webAuthProfiles = `	authorization {
		profile wb-admin {
			run { default-action allow }
			edit { default-action allow }
		}
		profile wb-reader {
			run { default-action allow }
			edit { default-action deny }
		}
	}
}
`

// WebAuthNeedsDaemon identifies declarations the standalone zefs power-user
// login cannot represent. The runtime runner and corpus MUST share this rule.
func WebAuthNeedsDaemon(users []webtesting.WBAuthUser) bool {
	if len(users) > 1 {
		return true
	}
	for _, user := range users {
		if user.Role == "" {
			continue
		}
		if user.Role != "admin" {
			return true
		}
	}
	return false
}

// WebDaemonWithAuth composes the parsed browser identities over WebDaemon's
// unchanged plugin/BGP baseline. The runtime runner and corpus MUST call this
// producer whenever a daemon browser case declares authentication. Passwords
// are synthetic fixture inputs; only their bcrypt hashes reach the config.
func WebDaemonWithAuth(ctx context.Context, plugins []string, users []webtesting.WBAuthUser) ([]byte, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return WebDaemon(plugins), nil
	}
	var body textbuf.Buffer
	body.Str("system {\n\tauthentication {\n")
	seen := make(map[string]bool, len(users))
	for _, user := range users {
		if err := context.Cause(ctx); err != nil {
			return nil, err
		}
		if user.Name == "" {
			return nil, fmt.Errorf("browser auth user requires a name; declare option=auth:user=<name>:password=<password>:role=admin|read-only")
		}
		if seen[user.Name] {
			return nil, fmt.Errorf("browser auth user %q is declared twice; give each fixture identity one auth declaration", user.Name)
		}
		seen[user.Name] = true
		profile := "wb-admin"
		switch user.Role {
		case "", "admin":
		case "read-only":
			profile = "wb-reader"
		default:
			return nil, fmt.Errorf("browser auth user %q has unknown role %q; declare role=admin or role=read-only", user.Name, user.Role)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("browser auth user %q: %w; use a fixture password of at most 72 bytes", user.Name, err)
		}
		body.Str("\t\tuser ").Str(strconv.Quote(user.Name)).Str(" {\n\t\t\tpassword ").Str(strconv.Quote(string(hash))).Str("\n\t\t\tprofile [ ").Str(profile).Str(" ]\n\t\t}\n")
	}
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	body.Str("\t}\n").Str(webAuthProfiles).Str(string(WebDaemon(plugins)))
	return []byte(body.String()), nil
}
