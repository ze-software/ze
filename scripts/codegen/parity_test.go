// The migration's proof for the five generators here: the scripts and the
// commands agree, on what they PRINT and on what they WRITE.
//
// scripts/codegen/*.go are being replaced by letools/{yangglue,pluginimports,
// featuretags,webassets,ianaasn}, and the two live side by side until the swap
// (plan/spec-le-is-a-ze-binary.md, step 14). This file is what makes that safe,
// and it is deliberately HERE rather than beside the new packages: it is a
// migration artifact, so it is deleted by the same commit that deletes the
// scripts it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree, each script and its
// command answer the same exit code and the same output, and for every WRITE
// mode they leave the same BYTES behind. A generator is not a checker: two
// halves that agree about "current" and disagree about what they emit would
// silently invalidate ze-generated-files-check for everyone.
// PREVENTS: a silent behavior change in a port that nobody reads the output of.
//
// THREE differences are DELIBERATE and are normalized rather than compared:
//
//   - The failure prefix and a program's own name. The scripts write
//     `yang_glue: `, `plugin_imports: `, `web_assets: ` and `feature_tags: `.
//     The commands keep the verdict wording and use `error: ` for a failure, the
//     spelling every ported le tool uses. normalize strips whichever it finds.
//   - How a file is named. The scripts variously print the absolute path they
//     had joined and, for a generated tag group, its bare base name. The
//     commands name every file RELATIVE to the tree, because an absolute path
//     names this machine's checkout as much as it names the file, and because
//     one payload cannot answer two path forms without leaving `| json` and the
//     default rendering disagreeing about what the value is. normalize reduces
//     both sides to the base-relative form.
//   - The stream a VERDICT lands on. Three of the four scripts print staleness
//     to stderr, because each models it as an error and exits through a fatal
//     path. A verdict is DATA in a command: it is the payload, so it lands on
//     stdout where `| json` can carry it, and only a genuine failure reaches
//     stderr. The two streams are therefore compared TOGETHER. What each stream
//     carries is not left unchecked -- it is a property of leroot.Run and
//     leaction.ReportError, which every ported tool shares and letools/leroot
//     tests directly.
//
// TWO fail-open behaviors of the scripts are pinned rather than reproduced, so
// each test goes red the day somebody fixes the script -- and the answer then is
// to delete the script and this file together:
// TestScriptPluginImportsStillDropsAFileItCannotRead and
// TestScriptIanaAsnStillWritesAnEmptyTable.

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/featuretags"
	"github.com/ze-software/ze/letools/ianaasn"
	"github.com/ze-software/ze/letools/pluginimports"
	"github.com/ze-software/ze/letools/webassets"
	"github.com/ze-software/ze/letools/yangglue"
)

// The two bounds this file needs. A link and a walk of a fixture tree are both
// sub-second on this hardware, so a run past either is a hung process rather
// than a slow one.
const (
	parityBuildTimeout = 180 * time.Second
	parityRunTimeout   = 120 * time.Second
)

// parityBinary maps a script file name to the binary built from it, once for
// the whole test binary. A per-case build would relink them for every fixture.
var parityBinary = map[string]string{}

// parityScripts are the five generators this file compares.
var parityScripts = []string{
	"yang_glue.go",
	"plugin_imports.go",
	"feature_tags.go",
	"web_assets.go",
	"iana_asn.go",
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "codegen-parity")
	if err != nil {
		panic("BUG: codegen parity test: cannot create a temporary directory")
	}
	code := 1
	if buildErr := buildCodegenScripts(dir); buildErr != nil {
		os.Stderr.WriteString(buildErr.Error() + "\n") //nolint:errcheck // test setup
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir) //nolint:errcheck // temporary directory
	os.Exit(code)     //nolint:gocritic // the defers this function would own live in buildCodegenScripts
}

// buildCodegenScripts compiles every script under test into dir. It is a
// function of its own so its context can be canceled by a defer: TestMain ends
// in os.Exit, which runs none.
func buildCodegenScripts(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), parityBuildTimeout)
	defer cancel()

	repo, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("resolve the repository root: %w", err)
	}

	for _, source := range parityScripts {
		binary := filepath.Join(dir, strings.TrimSuffix(source, ".go"))
		build := exec.CommandContext(ctx, "go", "build", "-o", binary, filepath.Join("scripts", "codegen", source))
		build.Dir = repo
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if combined, buildErr := build.CombinedOutput(); buildErr != nil {
			return fmt.Errorf("compile scripts/codegen/%s: %w\n%s", source, buildErr, combined)
		}
		parityBinary[source] = binary
	}

	return nil
}

// runScript runs one compiled generator over root, the way the gate invokes it:
// from inside the tree, which is how each script finds go.mod.
func runScript(t *testing.T, source, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	return runScriptWithEnv(t, source, root, nil, args...)
}

// runScriptWithEnv is runScript with extra environment entries, which is what
// the network generator's harness needs.
func runScriptWithEnv(t *testing.T, source, root string, extra []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parityBinary[source], args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), extra...)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s over %s: %v (%s)", source, root, err, errOut.String())
	}

	return out.String(), errOut.String(), code
}

// answer is what one half of a pair said: its two streams and its exit code.
type answer struct {
	stdout string
	stderr string
	code   int
}

// prose renders a command's answer the way leroot does for a bare invocation:
// the payload's own text on stdout, and the tool's verdict as the exit code.
func prose(text string, stale bool) answer {
	if stale {
		return answer{stdout: text, code: 1}
	}
	return answer{stdout: text}
}

// failure renders a command's answer for an error, the way leaction.ReportError
// does: one line on stderr, in the spelling every ported le tool uses.
func failure(err error) answer {
	return answer{stderr: "error: " + err.Error() + "\n", code: 1}
}

// normalize makes two runs over two temporary trees comparable: the tree's own
// path is replaced and then removed, and whichever program-name prefix a line
// carries is taken off. What is left is the message and the paths inside the
// tree.
func normalize(text, root string) string {
	text = strings.ReplaceAll(text, root, "<root>")
	text = strings.ReplaceAll(text, "<root>/", "")
	text = strings.ReplaceAll(text, "<root>", "")
	// The composition root's directory: the scripts name a generated group file
	// by its base name there, and the commands by its path.
	text = strings.ReplaceAll(text, "internal/component/plugin/all/", "")
	for _, prefix := range []string{"yang_glue: ", "plugin_imports: ", "feature_tags: ", "web_assets: ", "error: "} {
		text = strings.ReplaceAll(text, prefix, "")
	}
	return text
}

// compare asserts the script and the command answered the same thing. The two
// streams are compared together: see the deliberate differences at the top of
// this file.
func compare(t *testing.T, what string, script answer, scriptRoot string, command answer, commandRoot string) {
	t.Helper()

	if script.code != command.code {
		t.Errorf("%s: the script exited %d and the command exited %d\nscript: %s%s\ncommand: %s%s",
			what, script.code, command.code, script.stdout, script.stderr, command.stdout, command.stderr)
	}

	got := normalize(command.stdout+command.stderr, commandRoot)
	want := normalize(script.stdout+script.stderr, scriptRoot)
	if got != want {
		t.Errorf("%s: the output differs:\nscript:\n%s\ncommand:\n%s", what, want, got)
	}
}

// treeDigest answers a stable description of every file under root, so two
// trees can be compared without naming a file.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

	var out strings.Builder

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // a fixture tree this test built
		if readErr != nil {
			return readErr
		}
		out.WriteString(rel)
		out.WriteString("\x00")
		out.Write(body)
		out.WriteString("\x00")
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return out.String()
}

// writeFixture writes one file into a fixture tree, creating its parents.
func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture %s: %v", rel, err)
	}
}

// pairOfTrees builds one fixture twice, so the run that WRITES cannot see what
// the other run did.
func pairOfTrees(t *testing.T, build func(t *testing.T, root string)) (forScript, forCommand string) {
	t.Helper()

	forScript, forCommand = t.TempDir(), t.TempDir()
	for _, root := range []string{forScript, forCommand} {
		writeFixture(t, root, "go.mod", "module example.test/parity\n\ngo 1.26\n")
		writeFixture(t, root, "feature-gates.txt", "")
		build(t, root)
	}

	return forScript, forCommand
}

// checkThenWriteThenCheck drives one generator's whole pair over one fixture:
// the check both halves refuse, the write both halves perform, and the check
// both halves then pass. The two resulting trees must hold the same bytes.
type generatorPair struct {
	// script is the file name of the compiled generator.
	script string
	// writeArgs are the arguments its write mode takes; check mode adds
	// --check.
	writeArgs []string
	// check and write run the command half over one tree.
	check func(root string) answer
	write func(root string) answer
}

// scriptAnswer runs one script and packs its three results.
func (p generatorPair) scriptAnswer(t *testing.T, root string, args ...string) answer {
	t.Helper()

	out, errOut, code := runScript(t, p.script, root, args...)

	return answer{stdout: out, stderr: errOut, code: code}
}

// fixtureState is what a case declares about its own tree: whether the
// generator has work to do in it.
//
// It is declared rather than derived because a derived value would agree with
// the run by construction. A fixture can stop reaching its intended generator
// branch. Both halves then write nothing, and every comparison passes over two
// untouched trees. The declaration makes that result fail.
type fixtureState int

const (
	// fixtureCurrent is a tree with nothing to generate: the check passes and
	// the write leaves the same bytes behind.
	fixtureCurrent fixtureState = iota
	// fixtureStale is a tree that the check refuses and the write then fixes.
	// Only this state gives the byte comparison two results to compare.
	fixtureStale
	// fixtureRefused is a tree BOTH halves refuse: the generator cannot derive
	// its answer from it, so it writes nothing and says why. The comparison
	// there is of two refusals, and the tree is untouched on purpose.
	fixtureRefused
)

// String names the state in the words the failures use.
func (f fixtureState) String() string {
	switch f {
	case fixtureStale:
		return "stale"
	case fixtureRefused:
		return "refused"
	default:
		return "current"
	}
}

func (p generatorPair) run(t *testing.T, name string, state fixtureState, build func(t *testing.T, root string)) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		scriptRoot, commandRoot := pairOfTrees(t, build)
		checkArgs := append(append([]string{}, p.writeArgs...), "--check")

		before := treeDigest(t, scriptRoot)

		checkBefore := p.scriptAnswer(t, scriptRoot, checkArgs...)
		compare(t, "check before write", checkBefore, scriptRoot, p.check(commandRoot), commandRoot)
		if clean := checkBefore.code == 0; clean != (state == fixtureCurrent) {
			t.Errorf("this fixture is declared %s and the check before the write exited %d",
				state, checkBefore.code)
		}

		write := p.scriptAnswer(t, scriptRoot, p.writeArgs...)
		compare(t, "write", write, scriptRoot, p.write(commandRoot), commandRoot)
		if refused := write.code != 0; refused != (state == fixtureRefused) {
			t.Errorf("this fixture is declared %s and the write exited %d", state, write.code)
		}

		after := treeDigest(t, scriptRoot)
		if changed := after != before; changed != (state == fixtureStale) {
			t.Errorf("this fixture is declared %s and the write changed the tree: %v."+
				" A comparison of two trees neither generator wrote into proves nothing",
				state, changed)
		}

		if got, want := treeDigest(t, commandRoot), after; got != want {
			t.Error("the two trees differ after the write; the two generators do not emit the same bytes")
		}

		compare(t, "check after write",
			p.scriptAnswer(t, scriptRoot, checkArgs...), scriptRoot,
			p.check(commandRoot), commandRoot)
	})
}

// The four pairs whose gates this step ports. Each carries the command half's
// two entry points, called as functions rather than forked.
var (
	yangGluePair = generatorPair{
		script: "yang_glue.go",
		check: func(root string) answer {
			report, err := yangglue.Check(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), len(report.Stale) > 0)
		},
		write: func(root string) answer {
			report, err := yangglue.Write(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), false)
		},
	}

	pluginImportsPair = generatorPair{
		script: "plugin_imports.go",
		check: func(root string) answer {
			report, err := pluginimports.Check(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), report.Stale != "")
		},
		write: func(root string) answer {
			report, err := pluginimports.Write(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), false)
		},
	}

	featureTagsPair = generatorPair{
		script: "feature_tags.go",
		check: func(root string) answer {
			report, err := featuretags.Check(root)
			if err != nil {
				return failure(err)
			}
			// The script prints each stale file to stderr and the current
			// verdict to stdout; the command renders one payload, so the
			// verdict lands on stdout either way and normalize compares the
			// text. That is why this pair compares its streams together.
			return prose(report.Text(), len(report.Stale) > 0)
		},
		write: func(root string) answer {
			report, err := featuretags.Write(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), false)
		},
	}

	webAssetsPair = generatorPair{
		script: "web_assets.go",
		check: func(root string) answer {
			report, err := webassets.Check(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), report.Stale != "")
		},
		write: func(root string) answer {
			report, err := webassets.Write(root)
			if err != nil {
				return failure(err)
			}
			return prose(report.Text(), false)
		},
	}
)

// VALIDATES: the yang-glue script and command agree over a tree holding schema
// packages, over a tree holding none, and over the excluded directories.
// PREVENTS: glue generated for a different set of packages, or into the registry
// package the glue registers INTO.
func TestYangGlueAgrees(t *testing.T) {
	yangGluePair.run(t, "two schema packages and two exclusions", fixtureStale, func(t *testing.T, root string) {
		writeFixture(t, root, "internal/plugins/host/yang/ze-host-cmd.yang", "module ze-host-cmd {}\n")
		writeFixture(t, root, "internal/plugins/host/yang/ze-host.yang", "module ze-host {}\n")
		writeFixture(t, root, "internal/component/bgp/yang/ze-bgp.yang", "module ze-bgp {}\n")
		writeFixture(t, root, "internal/component/config/yang/ze-registry.yang", "module ze-registry {}\n")
		writeFixture(t, root, "internal/test/fake/yang/ze-fake.yang", "module ze-fake {}\n")
	})

	yangGluePair.run(t, "a tree with no schema package", fixtureCurrent, func(t *testing.T, root string) {
		writeFixture(t, root, "internal/component/keep.go", "package component\n")
	})

	yangGluePair.run(t, "a module name carrying every acronym shape", fixtureStale, func(t *testing.T, root string) {
		writeFixture(t, root, "internal/plugins/x/yang/ze-ospfv3.yang", "module ze-ospfv3 {}\n")
		writeFixture(t, root, "internal/plugins/x/yang/ze-flowexport.yang", "module ze-flowexport {}\n")
		writeFixture(t, root, "internal/plugins/x/yang/ze-unlisted-word.yang", "module ze-unlisted-word {}\n")
	})
}

// VALIDATES: the plugin-imports script and command agree over each discovery
// category, over a gated package, and over a tag file left behind by a gate
// that is gone.
// PREVENTS: a composition root that names a different set of packages, which is
// a feature that vanishes with no build error.
func TestPluginImportsAgrees(t *testing.T) {
	pluginImportsPair.run(t, "one package of each category", fixtureStale, func(t *testing.T, root string) {
		writeFixture(t, root, "feature-gates.txt", "ze_lg internal/plugins/looking\n")
		writeFixture(t, root, "internal/plugins/host/register.go", "package host\n")
		writeFixture(t, root, "internal/plugins/cli/register.go", "package cli\n\n// codegen:skip\n")
		writeFixture(t, root, "internal/plugins/host/yang/register.go",
			"package yang\n\nimport (\n\t_ \"example.test/parity/internal/component/config/yang\"\n)\n")
		writeFixture(t, root, "internal/component/config/yang/register.go",
			"package yang\n\nimport (\n\t_ \"example.test/parity/internal/component/config/yang\"\n)\n")
		writeFixture(t, root, "internal/component/ping/cmd/rpc.go",
			"package cmd\n\nfunc init() {\n\tpluginserver.RegisterRPCs(handlers)\n}\n")
		writeFixture(t, root, "internal/component/bgp/events.go",
			"package bgp\n\nfunc init() {\n\tevents.RegisterNamespace(\"bgp\")\n}\n")
		writeFixture(t, root, "internal/plugins/looking/register.go", "package looking\n")
		writeFixture(t, root, "internal/component/plugin/all/.keep", "")
	})

	pluginImportsPair.run(t, "a package nested under another gate", fixtureStale, func(t *testing.T, root string) {
		writeFixture(t, root, "feature-gates.txt",
			"ze_l2tp internal/component/l2tp\nze_radius internal/component/radius\n"+
				"ze_radius internal/component/l2tp/plugins/authradius\n")
		writeFixture(t, root, "internal/component/l2tp/plugins/authradius/register.go", "package authradius\n")
		writeFixture(t, root, "internal/plugins/radius/register.go", "package radius\n")
		writeFixture(t, root, "internal/component/plugin/all/.keep", "")
	})

	pluginImportsPair.run(t, "a generated tag file no gate needs", fixtureStale, func(t *testing.T, root string) {
		writeFixture(t, root, "internal/plugins/host/register.go", "package host\n")
		writeFixture(t, root, "internal/component/plugin/all/all_ze_gone.go",
			"// Code generated by scripts/codegen/plugin_imports.go; DO NOT EDIT.\n\n"+
				"//go:build ze_gone\n\npackage all\n")
	})
}

// VALIDATES: the feature-tags script and command agree over the four derived
// files, and refuse the same restructured file.
// PREVENTS: a gated package left unlinted, unshipped, or unanalysed by CodeQL
// because one of the four lists stopped naming its tag.
func TestFeatureTagsAgrees(t *testing.T) {
	featureTagsPair.run(t, "three gates, four stale files", fixtureStale, func(t *testing.T, root string) {
		writeFeatureTagsFixture(t, root,
			"ze_bgp internal/component/bgp\nze_web internal/component/web\nze_lg internal/component/lg\n",
			"linters:\n  build-tags:\n    - ze_core\n    - ze_bgp\n  enable:\n    - errcheck\n")
	})

	// This was named "already current" until 2026-08-26. The state declaration
	// above showed that the check refuses it because the linter file names the
	// gate, but the other three derived files do not.
	featureTagsPair.run(t, "one gate, named by the linter file alone", fixtureStale, func(t *testing.T, root string) {
		writeFeatureTagsFixture(t, root, "ze_bgp internal/component/bgp\n",
			"linters:\n  build-tags:\n    - ze_core\n    - ze_bgp\n  enable:\n    - errcheck\n")
	})

	featureTagsPair.run(t, "a golangci file whose key is gone", fixtureRefused, func(t *testing.T, root string) {
		writeFeatureTagsFixture(t, root, "ze_bgp internal/component/bgp\n",
			"linters:\n  enable:\n    - errcheck\n")
	})
}

// writeFeatureTagsFixture stands up the manifest and the four derived files.
func writeFeatureTagsFixture(t *testing.T, root, manifest, golangci string) {
	t.Helper()

	writeFixture(t, root, "feature-gates.txt", manifest)
	writeFixture(t, root, ".golangci.yml", golangci)
	writeFixture(t, root, filepath.Join("gokrazy", "ze", "config.json"),
		"{\n  \"Hostname\": \"ze\",\n  \"GoBuildTags\": [\"ze_core\", \"ze_appliance\"],\n  \"Packages\": []\n}\n")
	writeFixture(t, root, filepath.Join("docs", "guide", "quickstart.md"),
		"Install it:\n\n    CGO_ENABLED=0 go install -tags 'ze_core ze_distro' example.test/m/cmd/ze@latest\n")
	writeFixture(t, root, filepath.Join(".github", "workflows", "codeql.yml"),
		"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_distro' ./...\n"+
			"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_appliance' ./...\n"+
			"      - run: CGO_ENABLED=0 go build -tags 'ze_setup' ./cmd/ze\n")
}

// VALIDATES: the web-assets script and command agree over all three surfaces at
// once, and over markup the walk refuses.
// PREVENTS: a page whose head block loads an asset its markup no longer uses, or
// drops one it now needs, which is invisible everywhere but the browser.
func TestWebAssetsAgrees(t *testing.T) {
	webAssetsPair.run(t, "three surfaces, two page shapes", fixtureStale, writeWebAssetsFixture)

	webAssetsPair.run(t, "a shell that hand-writes its script tags", fixtureRefused, func(t *testing.T, root string) {
		writeWebAssetsFixture(t, root)
		writeFixture(t, root, "internal/component/web/layout.templ",
			"package web\n\ntempl layout() {\n\t<html><head><script src=\"/assets/htmx.min.js\"></script></head></html>\n}\n")
	})
}

// writeWebAssetsFixture stands up the three surfaces the generator derives: one
// shell naming its page at render time, one naming itself, and one whose markup
// is Go string literals.
func writeWebAssetsFixture(t *testing.T, root string) {
	t.Helper()

	writeFixture(t, root, "internal/component/web/layout.templ",
		"package web\n\n"+
			"//ze:page\ntempl peers() {\n\t<div hx-sse:connect=\"/events\">peers</div>\n\t@row()\n}\n\n"+
			"templ row() {\n\t<span hx-get=\"/row\">row</span>\n}\n\n"+
			"//ze:page\ntempl quiet() {\n\t<p>nothing moves here</p>\n}\n\n"+
			"templ layout(v View) {\n\t<html><head>@tags(pageAssets(v.Page))</head></html>\n}\n\n"+
			"templ tags(assets []string) {\n\t<span>tags</span>\n}\n")
	writeFixture(t, root, "internal/component/lg/layout.templ",
		"package lg\n\ntempl overview() {\n\t<html><head>@tags(pageAssets(pgOverview))</head>\n\t<div hx-get=\"/x\">x</div></html>\n}\n\n"+
			"templ tags(assets []string) {\n\t<span>tags</span>\n}\n")
	writeFixture(t, root, "internal/chaos/web/render.go",
		"package web\n\nfunc writeLayout() string {\n\treturn \"<html><head></head><div hx-get=\\\"/y\\\">y</div></html>\"\n}\n")
}

// The network generator's harness.
//
// iana_asn.go names five https:// URLs in a package-level table, so there is no
// seam a test can reach into. What IS reachable is the environment its HTTP
// client reads: http.DefaultTransport takes its proxy from HTTPS_PROXY, and
// crypto/x509 takes its root pool from SSL_CERT_FILE on Linux. So the harness
// below stands up a CONNECT proxy that terminates TLS itself, with a
// self-signed certificate naming all five hosts, and serves one fixture body to
// whatever the script asks for.
//
// That is the whole reason the comparison can happen offline and
// deterministically. The command half needs none of it: its fetch is a
// parameter, so the test hands it the same bytes directly.

// rirFixture is one registry's delegation file, in the format the five publish.
const rirFixture = "2|ripencc|20260826|3|0|20260826|+0000\n" +
	"ripencc|*|asn|*|3|summary\n" +
	"ripencc|EU|asn|1|2|19930901|allocated\n" +
	"ripencc|EU|asn|3|1|19930901|assigned\n" +
	"ripencc|EU|asn|100|1|19930901|reserved\n" +
	"ripencc|EU|ipv4|10.0.0.0|256|19930901|allocated\n"

// mitmProxy is a CONNECT proxy that answers every tunneled request with one
// body. It exists so a compiled script that speaks https to five fixed
// hostnames can be driven from a test with no network.
type mitmProxy struct {
	address  string
	certFile string
	body     string
}

// startMitmProxy stands the proxy up and stops it when the test ends.
func startMitmProxy(t *testing.T, body string) mitmProxy {
	t.Helper()

	certPEM, keyPEM := selfSignedCert(t)

	certFile := filepath.Join(t.TempDir(), "roots.pem")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write the root certificate: %v", err)
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load the server certificate: %v", err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}

	var listen net.ListenConfig
	listener, err := listen.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test teardown

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveTunnel(conn, config, body)
		}
	}()

	return mitmProxy{address: listener.Addr().String(), certFile: certFile, body: body}
}

// env answers the two variables a compiled script needs to reach this proxy.
func (p mitmProxy) env() []string {
	return []string{"HTTPS_PROXY=http://" + p.address, "SSL_CERT_FILE=" + p.certFile, "NO_PROXY="}
}

// serveTunnel answers one CONNECT, terminates TLS on the tunnel, and serves the
// body to whatever is asked for inside it.
func serveTunnel(conn net.Conn, config *tls.Config, body string) {
	defer conn.Close() //nolint:errcheck // test server

	reader := bufio.NewReader(conn)
	if _, err := http.ReadRequest(reader); err != nil {
		return
	}
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	inner := tls.Server(conn, config)
	defer inner.Close() //nolint:errcheck // test server

	innerReader := bufio.NewReader(inner)
	for {
		req, err := http.ReadRequest(innerReader)
		if err != nil {
			return
		}
		resp := &http.Response{
			Status:        "200 OK",
			StatusCode:    http.StatusOK,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Request:       req,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
		if err := resp.Write(inner); err != nil {
			return
		}
	}
}

// selfSignedCert answers a certificate that is its own root and names every
// delegation host, so a client trusting it accepts the proxy for all five.
func selfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "codegen parity"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames: []string{
			"ftp.ripe.net", "ftp.arin.net", "ftp.apnic.net", "ftp.afrinic.net", "ftp.lacnic.net",
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create the certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal the key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// generatedDate replaces the one line whose value is the day the run happened,
// so two runs a midnight apart still compare.
func generatedDate(text string) string {
	var out strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "// Generated: ") {
			out.WriteString("// Generated: <date>\n")
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

// irrTable is where both halves write, relative to the tree.
const irrTable = "internal/component/resolve/irr/rir_table.go"

// VALIDATES: given the same five delegation files, the iana-asn script and
// command write the same seed table, byte for byte.
// PREVENTS: a seed table whose ranges, collapse or interned constant names moved
// in the port. Nothing else in this repository reads that file closely enough to
// notice: it is data the resolver falls back to, and a wrong range answers the
// wrong registry rather than failing.
func TestIanaAsnWritesTheSameTable(t *testing.T) {
	proxy := startMitmProxy(t, rirFixture)

	scriptRoot, commandRoot := t.TempDir(), t.TempDir()
	for _, root := range []string{scriptRoot, commandRoot} {
		writeFixture(t, root, "go.mod", "module example.test/parity\n\ngo 1.26\n")
		writeFixture(t, root, filepath.Join(filepath.Dir(irrTable), ".keep"), "")
	}

	_, scriptErr, scriptCode := runScriptWithEnv(t, "iana_asn.go", scriptRoot, proxy.env())
	if scriptCode != 0 {
		t.Fatalf("the script exited %d: %s", scriptCode, scriptErr)
	}

	fetch := func(_ context.Context, _ string) ([]byte, error) { return []byte(rirFixture), nil }
	report, err := ianaasn.Write(context.Background(), commandRoot, fetch)
	if err != nil {
		t.Fatalf("the command failed: %v", err)
	}
	if report.Ranges != 1 || report.Records != 10 {
		t.Errorf("the command reported %+v, want 1 range from 10 records", report)
	}

	fromScript, err := os.ReadFile(filepath.Join(scriptRoot, irrTable))
	if err != nil {
		t.Fatalf("read the script's table: %v", err)
	}
	fromCommand, err := os.ReadFile(filepath.Join(commandRoot, irrTable))
	if err != nil {
		t.Fatalf("read the command's table: %v", err)
	}

	if got, want := generatedDate(string(fromCommand)), generatedDate(string(fromScript)); got != want {
		t.Errorf("the two tables differ:\nscript:\n%s\ncommand:\n%s", want, got)
	}
}

// VALIDATES: the script still writes an EMPTY seed table when the five
// registries answer with no ASN record, and still reports success.
// PREVENTS: this test going quietly out of date. It pins the fail-open the port
// closes, so it reddens the day somebody fixes the script -- and the answer then
// is to delete the script and this file together, not to relax the assertion.
func TestScriptIanaAsnStillWritesAnEmptyTable(t *testing.T) {
	proxy := startMitmProxy(t, "2|ripencc|20260826|0|0|20260826|+0000\n")

	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module example.test/parity\n\ngo 1.26\n")
	writeFixture(t, root, filepath.Join(filepath.Dir(irrTable), ".keep"), "")

	_, errOut, code := runScriptWithEnv(t, "iana_asn.go", root, proxy.env())
	if code != 0 {
		t.Fatalf("the script exited %d, want 0 -- has it been fixed? delete it and this file: %s", code, errOut)
	}

	body, err := os.ReadFile(filepath.Join(root, irrTable))
	if err != nil {
		t.Fatalf("the script wrote no table at all: %v", err)
	}
	if !strings.Contains(string(body), "var seedRIRTable = []RIREntry{\n}\n") {
		t.Fatalf("the script no longer writes an empty table -- has it been fixed? delete it and this file:\n%s", body)
	}

	// The port refuses the same input, which is the fix.
	fetch := func(_ context.Context, _ string) ([]byte, error) {
		return []byte("2|ripencc|20260826|0|0|20260826|+0000\n"), nil
	}
	if _, err := ianaasn.Write(context.Background(), t.TempDir(), fetch); err == nil {
		t.Error("the command accepted five answers holding no ASN record")
	}
}

// VALIDATES: the plugin-imports script still DROPS a schema package whose
// register.go it cannot read, and still reports the composition root current.
// PREVENTS: this test going quietly out of date. It pins the fail-open the port
// closes, so it reddens the day somebody fixes the script -- and the answer then
// is to delete the script and this file together, not to relax the assertion.
func TestScriptPluginImportsStillDropsAFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module example.test/parity\n\ngo 1.26\n")
	writeFixture(t, root, "feature-gates.txt", "")
	writeFixture(t, root, "internal/plugins/host/register.go", "package host\n")
	writeFixture(t, root, "internal/component/plugin/all/.keep", "")

	// Generate the composition root while the tree is entirely readable.
	if _, errOut, code := runScript(t, "plugin_imports.go", root); code != 0 {
		t.Fatalf("the script exited %d writing the composition root: %s", code, errOut)
	}

	// Now add a schema package the walk LISTS and cannot open. A dangling
	// symbolic link is permission-independent, so this holds for root as well.
	ghost := filepath.Join(root, "internal", "plugins", "ghost", "yang")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatalf("create the fixture directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(ghost, "register.go")); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}

	out, errOut, code := runScript(t, "plugin_imports.go", root, "--check")
	if code != 0 || !strings.Contains(out, "is current") {
		t.Fatalf("the script exited %d and said %q -- has it been fixed? delete it and this file: %s",
			code, out, errOut)
	}
	if !strings.Contains(out, "0 schemas") {
		t.Fatalf("the script no longer drops the unreadable schema package -- has it been fixed?\n%s", out)
	}

	// The port refuses the same tree, which is the fix.
	if _, err := pluginimports.Check(root); err == nil {
		t.Error("the command passed over a file it could not read")
	}
}
