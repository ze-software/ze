// Design: docs/architecture/appliance/docker-host.md -- docker-lab boots the cached docker-host image
// Related: workflowcheck_test.go -- workflowSource, jobBlocks, stripComments
// Related: qemu_nightly_own_system_test.go -- qemu-nightly docker-lab assertions

package workflowcheck

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const sharedDockerHostInputsCacheKey = "ze-docker-host-inputs-amd64-${{ hashFiles('gokrazy/docker-host/binaries.json', 'internal/le/setup/docker/host/static.go') }}"

// TestDockerLabJobsBuildAndCacheDockerHostImage pins every workflow job that
// boots Ze's docker-host image to building it first and sharing its non-sparse
// inputs cache across workflows.
//
// VALIDATES: every job in qemu-nightly.yml and evidence-nightly.yml that runs
// `./le test qemu docker-lab` runs `./le setup appliance-modules`, restores and
// saves the non-sparse downloads and static-binaries cache under one shared
// key, and runs `./le setup docker host build arch amd64` before `docker-lab`.
// PREVENTS: `docker-lab` refusing on a cold cache after `CachedImage` stopped
// building images inside test deadlines, and `actions/cache` expanding the
// 64 GiB sparse `docker-host.img` onto the runner disk.
func TestDockerLabJobsBuildAndCacheDockerHostImage(t *testing.T) {
	checked := 0
	for _, name := range []string{"qemu-nightly.yml", "evidence-nightly.yml"} {
		for _, job := range jobBlocks(t, name) {
			body := stripComments(job.body)
			labAt := strings.Index(body, "./le test qemu docker-lab")
			if labAt < 0 {
				continue
			}
			checked++
			checkDockerHostBuildSteps(t, name, job.name, body, labAt)
		}
	}
	if checked != 7 {
		t.Fatalf("checked %d docker-lab jobs across qemu-nightly.yml and evidence-nightly.yml, want 7", checked)
	}
}

func checkDockerHostBuildSteps(t *testing.T, workflow, job, body string, labAt int) {
	t.Helper()
	fillAt := strings.Index(body, "run: ./le setup appliance-modules")
	if fillAt < 0 {
		t.Errorf("%s job %q lacks ./le setup appliance-modules before docker-lab", workflow, job)
		return
	}
	buildAt := strings.Index(body, "run: ./le setup docker host build arch amd64")
	if buildAt < 0 {
		t.Errorf("%s job %q runs docker-lab without ./le setup docker host build arch amd64", workflow, job)
		return
	}
	if fillAt > buildAt {
		t.Errorf("%s job %q runs ./le setup appliance-modules after ./le setup docker host build arch amd64", workflow, job)
	}
	if buildAt > labAt {
		t.Errorf("%s job %q runs ./le setup docker host build arch amd64 after ./le test qemu docker-lab", workflow, job)
	}
	for _, required := range []string{
		"~/.cache/ze/docker-host/downloads",
		"~/.cache/ze/docker-host/amd64/static-*",
		sharedDockerHostInputsCacheKey,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("%s job %q lacks %q for the shared docker-host inputs cache", workflow, job, required)
		}
	}
}

// TestQEMURuntimeKernelLabsFillsModcacheBeforeL2TPProof pins step order in
// qemu-nightly.yml job runtime-kernel-labs.
//
// VALIDATES: `./le setup appliance-modules` and `./le setup docker host build
// arch amd64` both run as the runner user before `sudo -E ... gokrazy-l2tp-ppp`.
// PREVENTS: `gokrazy-l2tp-ppp` failing under GOPROXY=off against an empty
// module cache and leaving root-owned files in `gokrazy/modcache`.
func TestQEMURuntimeKernelLabsFillsModcacheBeforeL2TPProof(t *testing.T) {
	const name = "qemu-nightly.yml"
	var body string
	for _, job := range jobBlocks(t, name) {
		if job.name == "runtime-kernel-labs" {
			body = stripComments(job.body)
		}
	}
	if body == "" {
		t.Fatalf("%s has no runtime-kernel-labs job", name)
	}
	fillAt := strings.Index(body, "run: ./le setup appliance-modules")
	buildAt := strings.Index(body, "run: ./le setup docker host build arch amd64")
	l2tpAt := strings.Index(body, "./le test deployment gokrazy-l2tp-ppp")
	if fillAt < 0 {
		t.Fatalf("%s runtime-kernel-labs lacks ./le setup appliance-modules", name)
	}
	if buildAt < 0 {
		t.Fatalf("%s runtime-kernel-labs lacks ./le setup docker host build arch amd64", name)
	}
	if l2tpAt < 0 {
		t.Fatalf("%s runtime-kernel-labs lacks ./le test deployment gokrazy-l2tp-ppp", name)
	}
	if fillAt > l2tpAt {
		t.Errorf("%s runtime-kernel-labs runs ./le setup appliance-modules after gokrazy-l2tp-ppp", name)
	}
	if buildAt > l2tpAt {
		t.Errorf("%s runtime-kernel-labs runs ./le setup docker host build arch amd64 after gokrazy-l2tp-ppp", name)
	}
}

// TestEvidenceNightlyIntegrationLoadsFTPNATHelper pins the runner-kernel
// preparation required by TestConntrackHelperFTPThroughMasquerade.
//
// VALIDATES: evidence-nightly.yml job integration loads `nf_nat_ftp` with
// `sudo modprobe` before running `./le test integration firewall`.
// PREVENTS: TestConntrackHelperFTPThroughMasquerade failing on the Ubuntu
// runner kernel because `nft` auto-loads `nf_conntrack_ftp` but not `nf_nat_ftp`.
func TestEvidenceNightlyIntegrationLoadsFTPNATHelper(t *testing.T) {
	const name = "evidence-nightly.yml"
	var body string
	for _, job := range jobBlocks(t, name) {
		if job.name == "integration" {
			body = stripComments(job.body)
		}
	}
	if body == "" {
		t.Fatalf("%s has no integration job", name)
	}
	loadAt := strings.Index(body, "nf_nat_ftp")
	firewallAt := strings.Index(body, "./le test integration firewall")
	if loadAt < 0 {
		t.Fatalf("%s integration job does not load nf_nat_ftp", name)
	}
	if firewallAt < 0 {
		t.Fatalf("%s integration job lacks ./le test integration firewall", name)
	}
	if loadAt > firewallAt {
		t.Errorf("%s integration job loads nf_nat_ftp after ./le test integration firewall", name)
	}
	if !strings.Contains(body[:firewallAt], "sudo modprobe") {
		t.Errorf("%s integration job must load nf_nat_ftp via sudo modprobe", name)
	}
}

// TestCodeQLExtractionCheckGuardsEmptyDBLocations pins the step condition and
// JSON fallback on Check Go extraction evidence in codeql.yml.
//
// VALIDATES: the step requires success() and a non-empty db-locations output,
// and guards fromJSON against an empty string so GitHub Actions expression
// evaluation never crashes when the preceding build fails.
// PREVENTS: `Error reading JToken from JsonReader` masking a build failure in
// CodeQL's Go job.
func TestCodeQLExtractionCheckGuardsEmptyDBLocations(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				If   string            `yaml:"if"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflowSource(t, "codeql.yml")), &workflow); err != nil {
		t.Fatalf("parse codeql.yml: %v", err)
	}
	for _, step := range workflow.Jobs["analyze"].Steps {
		if step.Name != "Check Go extraction evidence" {
			continue
		}
		if !strings.Contains(step.If, "success()") {
			t.Errorf("codeql.yml %q if = %q, want success()", step.Name, step.If)
		}
		if !strings.Contains(step.If, "steps.analyze.outputs.db-locations != ''") {
			t.Errorf("codeql.yml %q if = %q, want steps.analyze.outputs.db-locations != ''", step.Name, step.If)
		}
		dbExpr := step.Env["CODEQL_DATABASE"]
		if !strings.Contains(dbExpr, "|| '{}'") {
			t.Errorf("codeql.yml %q CODEQL_DATABASE = %q, want empty-output JSON fallback", step.Name, dbExpr)
		}
		return
	}
	t.Fatal("codeql.yml has no Check Go extraction evidence step")
}
