// Design: docs/contributing/dependency-approval.md -- reuse declared modules;
// circl retains use-specific approval and new modules must land first.

package commit

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/deps"
)

func TestDependencyPolicyExistingModuleNeedsNoNewUseApproval(t *testing.T) {
	for _, requirement := range []string{
		"require golang.org/x/crypto v0.58.0\n",
		"require (\n\tgolang.org/x/crypto v0.58.0 // indirect\n)\n",
	} {
		root := dependencyBaseline(t, map[string]string{
			"go.mod":              "module fixture\n\n" + requirement,
			deps.LedgerRel:        "",
			deps.VendorModulesRel: "# golang.org/x/crypto v0.58.0\n## explicit\ngolang.org/x/crypto/ssh/knownhosts\n",
		})
		path := "internal/fresh/client.go"
		writeCommitFixture(t, root, path, "package fresh\n\nimport _ \"golang.org/x/crypto/ssh/knownhosts\"\n")
		if err := dependencyRefusal(root, []string{path}, nil, nil, "HEAD"); err != nil {
			t.Fatalf("existing module's new package/importer required approval: %v", err)
		}
	}
}

func TestDependencyPolicyCirclStillNeedsExactUseApproval(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{
		"go.mod":              "module fixture\n\nrequire github.com/cloudflare/circl v1.6.5\n",
		deps.LedgerRel:        "github.com/cloudflare/circl/sign/ed448\tinternal/approved\tapproved use\n",
		deps.VendorModulesRel: "# github.com/cloudflare/circl v1.6.5\n## explicit\ngithub.com/cloudflare/circl/sign/ed448\n",
	})
	for _, test := range []struct {
		path string
		pkg  string
		want bool
	}{
		{"internal/approved/client.go", "github.com/cloudflare/circl/sign/ed448", true},
		{"internal/fresh/client.go", "github.com/cloudflare/circl/sign/ed448", false},
		{"internal/approved/other.go", "github.com/cloudflare/circl/kem/mlkem", false},
	} {
		writeCommitFixture(t, root, test.path, "package client\n\nimport _ \""+test.pkg+"\"\n")
		err := dependencyRefusal(root, []string{test.path}, nil, nil, "HEAD")
		if (err == nil) != test.want {
			t.Fatalf("%s importing %s admitted=%v, want %v: %v", test.path, test.pkg, err == nil, test.want, err)
		}
		if !test.want && !strings.Contains(err.Error(), deps.ApproveCommand(test.pkg, deps.Importer(test.path))) {
			t.Fatalf("circl refusal lost owner-only approval command: %v", err)
		}
	}
}

func TestDependencyPolicyUncommittedModuleCannotAuthorizeAnImport(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{
		"go.mod":              "module fixture\n",
		deps.LedgerRel:        "",
		deps.VendorModulesRel: "",
	})
	path := "internal/fresh/client.go"
	writeCommitFixture(t, root, "go.mod", "module fixture\n\nrequire golang.org/x/crypto v0.58.0\n")
	writeCommitFixture(t, root, deps.VendorModulesRel, "# golang.org/x/crypto v0.58.0\n## explicit\ngolang.org/x/crypto/ssh/knownhosts\n")
	writeCommitFixture(t, root, path, "package fresh\n\nimport _ \"golang.org/x/crypto/ssh/knownhosts\"\n")
	for _, paths := range [][]string{{path}, {path, "go.mod", deps.VendorModulesRel}} {
		if err := dependencyRefusal(root, paths, nil, nil, "HEAD"); err == nil {
			t.Fatalf("uncommitted module authorized its first use in %v", paths)
		}
	}
}

func TestDependencyPolicyModulePathBoundaryIsExact(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{
		"go.mod":              "module fixture\n\nrequire golang.org/x/crypto v0.58.0\n",
		deps.LedgerRel:        "",
		deps.VendorModulesRel: "# golang.org/x/crypto v0.58.0\n## explicit\n",
	})
	path := "internal/fresh/client.go"
	writeCommitFixture(t, root, path, "package fresh\n\nimport _ \"golang.org/x/cryptography/other\"\n")
	if err := dependencyRefusal(root, []string{path}, nil, nil, "HEAD"); err == nil {
		t.Fatal("a different module sharing an approved module's prefix was admitted")
	}
}

func TestDependencyPolicyNestedModuleMustLandBeforeFirstUse(t *testing.T) {
	const parent = "example.org/approved"
	const nested = parent + "/plugin"
	const pkg = nested + "/client"
	for _, carryModule := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncarried-module", true: "carried-module"}[carryModule], func(t *testing.T) {
			root := dependencyBaseline(t, map[string]string{
				"go.mod":              "module fixture\n\nrequire " + parent + " v1.0.0\n",
				deps.LedgerRel:        "",
				deps.VendorModulesRel: "# " + parent + " v1.0.0\n## explicit\n" + parent + "/base\n",
			})
			writeCommitFixture(t, root, "go.mod", "module fixture\n\nrequire (\n\t"+parent+" v1.0.0\n\t"+nested+" v1.0.0\n)\n")
			writeCommitFixture(t, root, deps.VendorModulesRel, "# "+parent+" v1.0.0\n## explicit\n"+parent+"/base\n# "+nested+" v1.0.0\n## explicit\n"+pkg+"\n")
			path := "internal/fresh/client.go"
			writeCommitFixture(t, root, path, "package fresh\n\nimport _ \""+pkg+"\"\n")
			paths := []string{path}
			if carryModule {
				paths = append(paths, "go.mod", deps.VendorModulesRel)
			}
			if err := dependencyRefusal(root, paths, nil, nil, "HEAD"); err == nil {
				t.Fatalf("HEAD parent module %s authorized first use of new nested module %s in %v", parent, nested, paths)
			}
		})
	}
}

func TestDependencyPolicyExistingProviderAllowsPackageDescendants(t *testing.T) {
	const parent = "example.org/approved"
	const nested = parent + "/plugin"
	for _, provider := range []string{parent, nested} {
		t.Run(provider, func(t *testing.T) {
			requirements := "require " + parent + " v1.0.0\n"
			vendor := "# " + parent + " v1.0.0\n## explicit\n" + parent + "/base\n" + parent + "/client\n"
			if provider == nested {
				requirements += "require " + nested + " v1.0.0\n"
				vendor += "# " + nested + " v1.0.0\n## explicit\n" + nested + "/client\n"
			}
			root := dependencyBaseline(t, map[string]string{
				"go.mod":              "module fixture\n\n" + requirements,
				deps.LedgerRel:        "",
				deps.VendorModulesRel: vendor,
			})
			path := "internal/fresh/client.go"
			writeCommitFixture(t, root, path, "package fresh\n\nimport _ \""+provider+"/client\"\n")
			if err := dependencyRefusal(root, []string{path}, nil, nil, "HEAD"); err != nil {
				t.Fatalf("existing provider %s could not authorize its package descendant: %v", provider, err)
			}
		})
	}
}

func TestDependencyPolicyUncarriedRootModuleCannotHideCirclUse(t *testing.T) {
	const circl = "github.com/cloudflare/circl"
	const pkg = circl + "/sign/ed448"
	root := dependencyBaseline(t, map[string]string{
		"go.mod":              "module fixture\n\nrequire " + circl + " v1.6.5\n",
		deps.LedgerRel:        "",
		deps.VendorModulesRel: "# " + circl + " v1.6.5\n## explicit\n" + pkg + "\n",
	})
	writeCommitFixture(t, root, "go.mod", "module "+circl+"\n")
	path := "internal/fresh/client.go"
	writeCommitFixture(t, root, path, "package fresh\n\nimport _ \""+pkg+"\"\n")
	err := dependencyRefusal(root, []string{path}, nil, nil, "HEAD")
	if err == nil {
		t.Fatal("an uncarried root module rename hid a new circl use from approval")
	}
	if !strings.Contains(err.Error(), deps.ApproveCommand(pkg, deps.Importer(path))) {
		t.Fatalf("circl refusal lost its exact owner approval command: %v", err)
	}
}

func TestDependencyPolicySourceMembershipUsesCarriedNestedModule(t *testing.T) {
	for _, state := range []string{"uncarried", "carried", "existing"} {
		t.Run(state, func(t *testing.T) {
			const boundary = "examples/isolated/go.mod"
			const nestedMod = "module example.org/isolated\n"
			baseline := map[string]string{"go.mod": "module fixture\n", deps.LedgerRel: "", deps.VendorModulesRel: ""}
			if state == "existing" {
				baseline[boundary] = nestedMod
			}
			root := dependencyBaseline(t, baseline)
			writeCommitFixture(t, root, boundary, nestedMod)
			path := "examples/isolated/client.go"
			writeCommitFixture(t, root, path, "package isolated\n\nimport _ \"example.org/not-declared/client\"\n")
			paths := []string{path}
			if state == "carried" {
				paths = append(paths, boundary)
			}
			err := dependencyRefusal(root, paths, nil, nil, "HEAD")
			if state == "uncarried" {
				if err == nil {
					t.Fatal("an uncarried nested go.mod hid a carried root-module source import")
				}
			} else if err != nil {
				t.Fatalf("a genuinely separate %s module was judged as root-module source: %v", state, err)
			}
		})
	}
}

func TestDependencyPolicyVendorPackageCannotBorrowUnrelatedModule(t *testing.T) {
	const approved = "example.org/approved"
	const unrelated = "example.org/not-approved/client"
	root := dependencyBaseline(t, map[string]string{
		"go.mod":              "module fixture\n\nrequire " + approved + " v1.0.0\n",
		deps.LedgerRel:        "",
		deps.VendorModulesRel: "# " + approved + " v1.0.0\n## explicit\n" + approved + "/client\n",
	})
	// Go's package path contract joins the module path and its subdirectory.
	// A selected inventory line cannot extend that module's approval to an
	// unrelated import, even though the vendor reader trusts header grouping.
	writeCommitFixture(t, root, deps.VendorModulesRel, "# "+approved+" v1.0.0\n## explicit\n"+approved+"/client\n"+unrelated+"\n")
	path := "internal/fresh/client.go"
	writeCommitFixture(t, root, path, "package fresh\n\nimport _ \""+unrelated+"\"\n")
	err := dependencyRefusal(root, []string{path, deps.VendorModulesRel}, nil, nil, "HEAD")
	if err == nil {
		t.Fatal("selected vendor package outside its provider namespace borrowed an existing module's approval")
	}
	if !strings.Contains(err.Error(), deps.ApproveCommand(unrelated, deps.Importer(path))) {
		t.Fatalf("unrelated import refusal lost its exact owner approval command: %v", err)
	}
}
