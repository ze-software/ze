//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- authenticated corpus composition.

package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/web"
)

// browserCorpusAuthentication is the parsed fixture's local login authority.
// Its constructor requires both authentication users and an authorization
// store. The zero value is not admitted; a nil pointer means the workflow
// declares no authentication and uses the runner's insecure fixture instead.
type browserCorpusAuthentication struct {
	local  authz.LocalAuthenticator
	policy *authz.Store
}

// browserCorpusIdentity exists only after a successful fixture login. Its
// private profile snapshot preserves the original authenticated caller while
// navigation and candidate edits leave that caller's authority unchanged.
// Neither its zero value nor a nil pointer is an authenticated owner.
type browserCorpusIdentity struct {
	user     string
	password string //nolint:gosec // Fixture credential retained only for native in-memory web authentication.
	profiles []string
	policy   *authz.Store
}

func newBrowserCorpusAuthentication(schema *config.Schema, baseline []byte) (*browserCorpusAuthentication, error) {
	tree, err := config.NewParser(schema).Parse(string(baseline))
	if err != nil {
		return nil, err
	}
	system := tree.GetContainer("system")
	if system == nil {
		return nil, errors.New("declared browser authentication has no system configuration")
	}
	users := infra.ExtractAuthUsers(system.ToMap())
	if len(users) == 0 {
		return nil, errors.New("declared browser authentication has no local users")
	}
	policy, err := infra.ExtractAuthzStore(tree)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, errors.New("declared browser authentication has no authorization store")
	}
	return &browserCorpusAuthentication{local: authz.LocalAuthenticator{Users: users}, policy: policy}, nil
}

func (a *browserCorpusAuthentication) login(user, password string) (*browserCorpusIdentity, error) {
	result, err := a.local.Authenticate(authz.AuthRequest{Username: user, Password: password, Service: "web"})
	if err != nil {
		return nil, fmt.Errorf("browser authentication failed for %q: %w", user, err)
	}
	if !result.Authenticated {
		return nil, fmt.Errorf("browser authentication failed for %q", user)
	}
	return &browserCorpusIdentity{user: user, password: password, profiles: slices.Clone(result.Profiles), policy: a.policy}, nil
}

func (b *browserCorpus) login(user, password string) error {
	if b.auth == nil {
		return errors.New("browser login has no declared authentication fixture")
	}
	if b.identity != nil {
		return errors.New("browser is already authenticated; the runner cannot fill a second login form")
	}
	identity, err := b.auth.login(user, password)
	if err != nil {
		return err
	}
	b.identity = identity
	return nil
}

func (b *browserCorpus) authorize(line string) error {
	if b.auth == nil {
		return nil
	}
	if b.identity == nil {
		return errors.New("browser authentication required before configuration composition")
	}
	permission, err := web.ConfigCommandPermission(line)
	if err != nil {
		return err
	}
	if permission == "" {
		return nil // Navigation and read-only commands have no editor mutation gate.
	}
	return b.identity.authorize(permission)
}

func (identity *browserCorpusIdentity) authorize(permission string) error {
	if identity.policy.AuthorizeWithProfiles(identity.user, identity.profiles, permission, false) == authz.Deny {
		return fmt.Errorf("browser authorization denied %q for %q", permission, identity.user)
	}
	return nil
}
