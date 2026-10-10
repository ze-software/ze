package appliance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

const kernelBuilderPkg = "internal/appliance/kernelbuilder"

// hashedBuilderSources answers the files the cached-kernel key hashes, from
// the repository root.
func hashedBuilderSources(t *testing.T) map[string]bool {
	t.Helper()
	paths, ok := kernelBuilderSources()
	if !ok {
		t.Fatal("kernelBuilderSources found no build inputs from the repository root")
	}
	hashed := make(map[string]bool, len(paths))
	for _, path := range paths {
		hashed[filepath.ToSlash(path)] = true
	}
	return hashed
}

// TestKernelCacheKeyCoversEveryBuildInput proves the cached-kernel key hashes
// every input that decides a kernel's bytes: every file under
// tools/kernel-builder, every BuildSources file, every Ze package those Go files
// import, and that no BuildSources file reaches a value or function declared in
// a host-side kernelbuilder file, where an edit would change the build and keep
// the key.
//
// VALIDATES: round-2 NOTE 1, the key covers what reaches the build.
// PREVENTS: a stale kernel served after an edit to code the build runs.
func TestKernelCacheKeyCoversEveryBuildInput(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	hashed := hashedBuilderSources(t)

	err := filepath.WalkDir(kernelBuilderDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && !hashed[filepath.ToSlash(path)] {
			t.Errorf("the key leaves out %s, which the build container is made from", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	buildFiles := make(map[string]bool, len(kernelbuilder.BuildSources))
	for _, name := range kernelbuilder.BuildSources {
		buildFiles[name] = true
		if !hashed[kernelBuilderPkg+"/"+name] {
			t.Errorf("the key leaves out %s/%s, a BuildSources file", kernelBuilderPkg, name)
		}
	}

	// Each hashed Go file's Ze imports are hashed whole: one level, checked over
	// the whole hashed set, is the transitive closure.
	fset := token.NewFileSet()
	for path := range hashed {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, spec := range file.Imports {
			dir, inModule := strings.CutPrefix(strings.Trim(spec.Path.Value, `"`), "github.com/ze-software/ze/")
			if !inModule || dir == kernelBuilderPkg {
				continue
			}
			for _, goFile := range nonTestGoFiles(t, dir) {
				if !hashed[goFile] {
					t.Errorf("%s imports %s, and the key leaves out %s", path, dir, goFile)
				}
			}
		}
	}

	// No BuildSources file may use anything a host-side file declares: a
	// function, a variable, a constant, a type, a method or a field. Resolved
	// by the type checker, so a method or a field is caught as surely as a
	// function call.
	pkg := loadKernelBuilder(t)
	held := map[string]bool{}
	for _, file := range pkg.Syntax {
		held[filepath.Base(pkg.Fset.Position(file.Pos()).Filename)] = true
	}
	for name := range buildFiles {
		if !held[name] {
			t.Errorf("BuildSources names %s, which %s does not hold", name, kernelBuilderPkg)
		}
	}
	for _, file := range pkg.Syntax {
		name := filepath.Base(pkg.Fset.Position(file.Pos()).Filename)
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Recv == nil && fn.Name.Name == "init" && !buildFiles[name] {
				t.Errorf("%s declares init, which runs in the build container, outside BuildSources", name)
			}
		}
		if !buildFiles[name] {
			continue
		}
		for ident, obj := range pkg.TypesInfo.Uses {
			if pkg.Fset.Position(ident.Pos()).Filename != pkg.Fset.Position(file.Pos()).Filename {
				continue
			}
			if obj.Pkg() == nil || obj.Pkg() != pkg.Types {
				continue
			}
			if home := filepath.Base(pkg.Fset.Position(obj.Pos()).Filename); !buildFiles[home] {
				t.Errorf("%s/%s uses %s, declared in the host-side %s, which the key does not hash: move it into a BuildSources file", kernelBuilderPkg, name, obj.Name(), home)
			}
		}
	}
}

// loadKernelBuilder type-checks the kernelbuilder package, without its tests,
// from the repository root.
func loadKernelBuilder(t *testing.T) *packages.Package {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}, "./"+kernelBuilderPkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %v", kernelBuilderPkg, pkgs)
	}
	return pkgs[0]
}

// TestHostSideNeverWritesTheRequest proves the other direction of the key: a
// host-side kernelbuilder file never decides what the build receives. The
// Request is written only by normalizeRequest (recipe.go), and the backends
// run only the recipe's argv (TestRunDockerArgvAndOwnershipRepair,
// TestBuildGuestWorkerTakesTheRecipe, and the sshRun check below), so a
// host-side edit cannot change a kernel's bytes and keep the key.
//
// VALIDATES: round-3 ISSUE 1, the guard is two-way.
// PREVENTS: a host-side rewrite of the request (a default, a cleaned path, a
// modules value) changing the build under an unchanged key.
func TestHostSideNeverWritesTheRequest(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	pkg := loadKernelBuilder(t)
	buildFiles := map[string]bool{}
	for _, name := range kernelbuilder.BuildSources {
		buildFiles[name] = true
	}
	request := pkg.Types.Scope().Lookup("Request")
	if request == nil {
		t.Fatal("kernelbuilder declares no Request")
	}
	isRequest := func(expr ast.Expr) bool {
		typ := pkg.TypesInfo.TypeOf(expr)
		if pointer, isPointer := typ.(*types.Pointer); isPointer {
			typ = pointer.Elem()
		}
		return typ != nil && types.Identical(typ, request.Type())
	}
	// writesRequest answers whether expr, the target of a write, is the Request
	// or one of its fields.
	writesRequest := func(expr ast.Expr) bool {
		for {
			switch node := expr.(type) {
			case *ast.SelectorExpr:
				if isRequest(node.X) {
					return true
				}
				expr = node.X
			case *ast.IndexExpr:
				expr = node.X
			case *ast.StarExpr:
				return isRequest(node.X)
			case *ast.ParenExpr:
				expr = node.X
			default:
				return false
			}
		}
	}
	for _, file := range pkg.Syntax {
		name := filepath.Base(pkg.Fset.Position(file.Pos()).Filename)
		if buildFiles[name] {
			continue
		}
		var sshCommands int
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if writesRequest(lhs) {
						t.Errorf("%s writes the build request at %s: only normalizeRequest (recipe.go) may", name, pkg.Fset.Position(lhs.Pos()))
					}
				}
			case *ast.IncDecStmt:
				if writesRequest(node.X) {
					t.Errorf("%s writes the build request at %s: only normalizeRequest (recipe.go) may", name, pkg.Fset.Position(node.Pos()))
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND && writesRequest(node.X) {
					t.Errorf("%s hands out a pointer into the build request at %s: only normalizeRequest (recipe.go) may write it", name, pkg.Fset.Position(node.Pos()))
				}
			case *ast.CompositeLit:
				if isRequest(node) {
					t.Errorf("%s builds a Request of its own at %s: the build receives the caller's request, normalised by recipe.go", name, pkg.Fset.Position(node.Pos()))
				}
			case *ast.CallExpr:
				if callee, isIdent := node.Fun.(*ast.Ident); isIdent && callee.Name == "sshRun" && len(node.Args) > 2 {
					sshCommands++
					command, isCall := node.Args[2].(*ast.CallExpr)
					recipe, named := ast.Expr(nil), false
					if isCall {
						recipe = command.Fun
						ident, isIdent := recipe.(*ast.Ident)
						named = isIdent && ident.Name == "guestBuildCommand"
					}
					if !named {
						t.Errorf("%s runs a guest command at %s that is not guestBuildCommand(...) itself: the guest runs the recipe's command, unchanged", name, pkg.Fset.Position(node.Pos()))
					}
				}
			}
			return true
		})
		if name == "qemu.go" && sshCommands == 0 {
			t.Errorf("qemu.go runs no guest command through sshRun, so this check sees no QEMU build")
		}
	}
}

// TestKernelCacheKeyCarriesTheBackend proves a Docker kernel and a QEMU kernel
// never share a cache entry: the two compile with different gcc releases.
//
// VALIDATES: round-3 ISSUE 2, the backend is keyed.
// PREVENTS: a kernel one backend built served for a build the other would run.
func TestKernelCacheKeyCarriesTheBackend(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	resolved, err := resolveKernelProfile(runtimeKernelConfigDir, runtimeKernelProfile)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, builder := range []string{builderDocker, builderQEMU, ""} {
		key, err := kernelCacheVariantFor(kernelTargetRuntime, archAMD64, builder, resolved)
		if err != nil {
			t.Fatal(err)
		}
		keys[builder] = key
	}
	if keys[builderDocker] == keys[builderQEMU] {
		t.Errorf("docker and qemu kernels share the key %s", keys[builderDocker])
	}
	if want := keys[kernelbuilder.PreferredBackend("")]; keys[""] != want {
		t.Errorf("a build naming no backend keys %s, want the key of the backend it would run, %s", keys[""], want)
	}
}

// TestKernelCacheKeyRefusesWhatItCannotHash proves a key that cannot be
// computed is refused with its fix, never replaced by a shorter key every
// input would share.
//
// VALIDATES: round-3 NOTE 2.
// PREVENTS: one kernel served for every config when the hash inputs are absent.
func TestKernelCacheKeyRefusesWhatItCannotHash(t *testing.T) {
	t.Chdir(t.TempDir())
	resolved := kernelProfileResolution{Name: runtimeKernelProfile, Fragments: []string{"fixture.config"}}
	patch := filepath.Join(runtimeKernelPatchesDir, "0001.patch")
	// Each step supplies the input the previous one lacked, so every refusal
	// is reached in turn: the patches, the config, then the builder sources.
	steps := []struct {
		missing string
		supply  string
	}{
		{"the patch directory", patch},
		{"its config", "fixture.config"},
		{"the builder sources", ""},
	}
	for _, step := range steps {
		key, err := kernelCacheVariantFor(kernelTargetRuntime, archAMD64, "", resolved)
		if err == nil {
			t.Fatalf("kernelCacheVariantFor without %s = %q, want a refusal", step.missing, key)
		}
		if !strings.Contains(err.Error(), "cannot key the runtime kernel cache") || !strings.Contains(err.Error(), "run ze appliance from the top of the Ze source tree") {
			t.Errorf("refusal without %s = %v, want the cause and the fix", step.missing, err)
		}
		if step.supply == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(step.supply), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(step.supply, []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestKernelCacheKeyIgnoresHostSideEdits proves an edit to a host-side
// kernelbuilder file (a probe, a measurement, a refusal's wording) keeps the
// key, and an edit to a BuildSources file changes it. It runs over a copy of
// the package, beside the real tools/kernel-builder and Ze packages.
//
// VALIDATES: round-2 NOTE 1, a probe or message edit stales no cached kernel.
// PREVENTS: a thirty-minute rebuild per arch after a wording change.
func TestKernelCacheKeyIgnoresHostSideEdits(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mirror := t.TempDir()
	for _, name := range []string{"go.mod", "tools"} {
		if err := os.Symlink(filepath.Join(root, name), filepath.Join(mirror, name)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(mirror, filepath.FromSlash(kernelBuilderPkg))
	if err := os.MkdirAll(pkg, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "appliance" {
			continue
		}
		if err := os.Symlink(filepath.Join(root, "internal", entry.Name()), filepath.Join(mirror, "internal", entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range nonTestGoFiles(t, filepath.Join(root, filepath.FromSlash(kernelBuilderPkg))) {
		data, readErr := os.ReadFile(path) //nolint:gosec // test reads the package it mirrors
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(filepath.Join(pkg, filepath.Base(path)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(mirror)
	key := func() string {
		paths, ok := kernelBuilderSources()
		if !ok {
			t.Fatal("kernelBuilderSources found no build inputs in the mirror")
		}
		hash, ok := cacheFileHashPaths(paths)
		if !ok {
			t.Fatal("cacheFileHashPaths could not read a build input in the mirror")
		}
		return hash
	}
	edit := func(name string) {
		file, openErr := os.OpenFile(filepath.Join(pkg, name), os.O_APPEND|os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, err := file.WriteString("\n// an edit\n"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	before := key()
	var hostSide []string
	for _, path := range nonTestGoFiles(t, pkg) {
		if name := filepath.Base(path); !slices.Contains(kernelbuilder.BuildSources, name) {
			hostSide = append(hostSide, name)
			edit(name)
		}
	}
	for _, want := range []string{"space.go", "qemu.go", "driver.go"} {
		if !slices.Contains(hostSide, want) {
			t.Errorf("%s is not host-side, so the probes and refusals it holds would stale every kernel", want)
		}
	}
	if after := key(); after != before {
		t.Errorf("an edit to the host-side files %v changed the key %s to %s", hostSide, before, after)
	}
	for _, name := range kernelbuilder.BuildSources {
		previous := key()
		edit(name)
		if key() == previous {
			t.Errorf("an edit to %s, a BuildSources file, kept the key", name)
		}
	}
}

func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.FromSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, filepath.ToSlash(filepath.Join(dir, name)))
		}
	}
	return files
}

// mustKernelCacheVariant is kernelCacheVariant for a test, which has the tree.
func mustKernelCacheVariant(t *testing.T, arch, profile string) string {
	t.Helper()
	variant, err := kernelCacheVariant(arch, profile)
	if err != nil {
		t.Fatalf("kernelCacheVariant(%s, %s): %v", arch, profile, err)
	}
	return variant
}

// mustKernelCacheVariantFor is kernelCacheVariantFor for a build that names
// no backend.
func mustKernelCacheVariantFor(t *testing.T, target, arch string, profile kernelProfileResolution) string {
	t.Helper()
	variant, err := kernelCacheVariantFor(target, arch, "", profile)
	if err != nil {
		t.Fatalf("kernelCacheVariantFor(%s, %s): %v", target, arch, err)
	}
	return variant
}
