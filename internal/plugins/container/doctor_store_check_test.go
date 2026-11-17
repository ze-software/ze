// Design: docs/architecture/container/image-store.md -- the doctor check over the store root

package container

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkedStore is a store whose check answers refusal. It resolves nothing:
// the doctor check never asks it to.
type checkedStore struct {
	refusal error
}

func (checkedStore) Resolve(context.Context, ResolveRequest) (Resolution, error) {
	return nil, errors.New("checkedStore resolves nothing")
}

func (checkedStore) Hold(string, Ready) func() { return func() {} }

func (c checkedStore) CheckStore() error { return c.refusal }

// uncheckedStore is a store with no folder of its own, as the runtime's
// fixture store is: it is no StoreChecker.
type uncheckedStore struct{}

func (uncheckedStore) Resolve(context.Context, ResolveRequest) (Resolution, error) {
	return nil, errors.New("uncheckedStore resolves nothing")
}

func (uncheckedStore) Hold(string, Ready) func() { return func() {} }

// TestStoreDoctorCheckAnswersTheStoresRefusal proves the runtime check
// container-store is declared at Stage 1 under the store-unwritable code and
// answers through the doctor callback what the store's own check answers:
// nothing for a usable root, one error finding with the store's problem,
// cause and fix for a refused one, and a refusal of its own for a store that
// answers with no fix.
//
// Method: engineRegistration's declarations, then answerDoctorCheck on
// supervisors built over stores whose check answers each case, and over a
// store that keeps no folder.
//
// MUTATION: drop storeDoctorChecks from runtimeDoctorCheckDecls and the check
// is not declared; answer no finding in answerStoreUsable and the refused
// root is not reported.
func TestStoreDoctorCheckAnswersTheStoresRefusal(t *testing.T) {
	const name = "container-store"
	var decl *rpc.DoctorCheckDecl
	for _, declared := range engineRegistration().DoctorChecks {
		if declared.Name == name {
			decl = &declared
		}
	}
	if decl == nil {
		t.Fatalf("check %s is not declared in the Stage 1 registration", name)
	}
	if decl.Phase != rpc.DoctorPhaseRuntime {
		t.Errorf("check %s is declared at phase %s, want %s", name, decl.Phase, rpc.DoctorPhaseRuntime)
	}
	if len(decl.Codes) != 1 || decl.Codes[0] != codeStoreUnwritable {
		t.Fatalf("check %s declares codes %v, want [%s]", name, decl.Codes, codeStoreUnwritable)
	}
	if diagnostic.Lookup(codeStoreUnwritable) == nil {
		t.Errorf("code %s is not in the code registry", codeStoreUnwritable)
	}

	answer := func(store Store) ([]rpc.DoctorCheckDiagnostic, error) {
		t.Helper()
		sup := newSupervisor(slogutil.DiscardLogger(), nil, func(StoreNotices) Store { return store }, nil)
		return sup.answerDoctorCheck(name)
	}

	healthy, err := answer(checkedStore{})
	if err != nil {
		t.Fatalf("a usable store root was refused: %v", err)
	}
	if len(healthy) != 0 {
		t.Errorf("a usable store root answered %v, want no finding", healthy)
	}

	cause := errors.New("permission denied")
	refusal := opererr.Wrap(cause,
		"the container store /data/container is open to group or others",
		"an image holds set-user files",
		"chmod 0700 /data/container",
	)
	findings, err := answer(checkedStore{refusal: refusal})
	if err != nil {
		t.Fatalf("the check refused instead of reporting the store's refusal: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("a refused store root answered %d findings, want 1: %v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != codeStoreUnwritable {
		t.Errorf("the finding's code is %s, want %s", finding.Code, codeStoreUnwritable)
	}
	if finding.Severity != severityError {
		t.Errorf("the finding's severity is %q, want %q", finding.Severity, severityError)
	}
	for _, part := range []string{refusal.Problem(), refusal.Why(), cause.Error()} {
		if !strings.Contains(finding.Message, part) {
			t.Errorf("the finding's message %q does not hold %q", finding.Message, part)
		}
	}
	if finding.Fix != refusal.Fix() {
		t.Errorf("the finding's fix is %q, want the store's %q", finding.Fix, refusal.Fix())
	}

	if _, err := answer(checkedStore{refusal: errors.New("no fix")}); err == nil {
		t.Error("a store check that answered no operator error was reported as a finding, want a refusal naming the defect")
	}

	// The guard: a store that keeps no folder has no root to be unusable.
	none, err := answer(uncheckedStore{})
	if err != nil {
		t.Fatalf("a store with no folder of its own was refused: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("a store with no folder of its own answered %v, want no finding", none)
	}
}
