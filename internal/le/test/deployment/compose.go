// Design: docs/guide/docker.md -- the Compose journey this proof walks
// Overview: deployment.go -- the shared proof machinery
// Related: composessh.go -- the authenticated SSH sessions the proof opens
// Related: composereport.go -- the payload a run answers
//
// compose.go proves AC-5 of the deployment journeys: a container started from
// the supplied docker/compose.yaml keeps an operator's authenticated
// configuration change and login across a new container on the same named
// volume. Every step is a command the Docker guide gives a user: `compose
// build`, `run --rm ze init`, `run --rm -T ze config import`, `up -d`, an SSH
// login on the published port, a commit in the SSH editor, `down`, `up -d`.
//
// Two changes are merged over the user's file, both forced by a Docker daemon
// many sessions share: the image reference (a run-unique tag for a tree build,
// or the prepared preview image) and the published ports (one loopback port
// the daemon picks for SSH, rather than the fixed 2222 and 179 another run may
// hold). The project name is run-unique, so every container, network and
// volume is this run's own, and teardown removes exactly those by name. The
// container, the network and the named volume carry the owner labels. The lab
// cleanup removes no volume, so a SIGKILLed run's store is named, with its
// removal command, by the next run's notice (noticeEarlierVolumes).
//
// Two counterfactuals each fail at their own step: break-volume mounts no
// volume, so init's store goes with its one-off container and the run fails
// at import; break-commit imports docker/ze.conf again between the
// containers, so only second-login, the persistence assertion, goes red.

package testdeployment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// composeService is the one service docker/compose.yaml declares.
	composeService = "ze"
	// composeSSHPort is the listener docker/ze.conf enables in the container.
	composeSSHPort = 2222
	// composeUser is the operator the proof initializes the store with.
	composeUser = "operator"
	// composeEditValue is the value the operator commits at composeEditPath.
	composeEditValue = "warn"
	// composeBuiltRepository names the image a tree build tags; the tag is the
	// run's project name, so no other run's image is ever replaced.
	composeBuiltRepository = "ze-compose-proof"
	// composeLoginTimeout bounds the wait for the daemon's SSH listener after
	// `up`: the daemon starts its plugins first.
	composeLoginTimeout = 2 * time.Minute
	// composeApplyTimeout bounds the wait for a commit to reach the running
	// configuration.
	composeApplyTimeout = time.Minute
)

// composeEditPath is the leaf the operator sets: one docker/ze.conf does not
// set, so the committed value there can only come from the commit.
var composeEditPath = []string{"environment", "log", "level"}

// composeProof is one run of the Compose persistence proof.
type composeProof struct {
	root   string
	docker *interoplab.Docker
	// image is the prepared image to run; empty builds docker/Dockerfile from
	// the checkout under a run-unique tag.
	image string
	// breakVolume is the counterfactual: the service mounts no volume, so the
	// store lives in each container's own layer and nothing persists.
	breakVolume bool
	// breakCommit is the counterfactual for the persistence assertion itself:
	// between the two containers docker/ze.conf is imported again over the
	// committed change, keeping the credential and the host key, so the run
	// must fail at second-login and nowhere earlier.
	breakCommit bool
	project     interoplab.ComposeProject
	progress    io.Writer
	password    string
}

// composeVolume is the named volume docker/compose.yaml mounts at the store.
const composeVolume = "ze-data"

// newComposeProof answers a run over the checkout at root with a fresh
// project name and operator password.
func newComposeProof(root string, docker *interoplab.Docker, image string, breakVolume, breakCommit bool, progress io.Writer) (*composeProof, error) {
	suffix, err := randomHex(4)
	if err != nil {
		return nil, err
	}
	password, err := randomHex(12)
	if err != nil {
		return nil, err
	}
	return &composeProof{
		root: root, docker: docker, image: image, breakVolume: breakVolume, breakCommit: breakCommit, progress: progress,
		project:  interoplab.ComposeProject{Name: interoplab.ComposeProjectPrefix + suffix},
		password: password,
	}, nil
}

func randomHex(octets int) (string, error) {
	buf := make([]byte, octets)
	if _, err := rand.Read(buf); err != nil {
		return "", errors.New("read random bytes for a run-unique name: " + err.Error())
	}
	return hex.EncodeToString(buf), nil
}

// builtImage is the tag a tree build gives the image, or empty when the run
// uses a supplied image it does not own.
func (p *composeProof) builtImage() string {
	if p.image != "" {
		return ""
	}
	return composeBuiltRepository + ":" + p.project.Name
}

// runImage is the image every container of the project runs.
func (p *composeProof) runImage() string {
	if p.image != "" {
		return p.image
	}
	return p.builtImage()
}

// override answers the Compose file merged over docker/compose.yaml. The
// named volume carries the owner labels, so `./le test integration clean`
// finds the store a killed run left. The volume counterfactual mounts no
// volume, so it labels none.
func (p *composeProof) override() (string, error) {
	volume := composeVolume
	if p.breakVolume {
		volume = ""
	}
	return p.docker.ComposeOverrideFile(interoplab.ComposeOverride{
		Service:     composeService,
		Image:       p.runImage(),
		Ports:       []interoplab.ContainerPort{interoplab.TCPPort(composeSSHPort)},
		DropVolumes: p.breakVolume,
		Volume:      volume,
	})
}

// Run walks the journey and answers what it saw. An error is a step the proof
// could not perform; a step that ran and disproved persistence is the verdict.
func (p *composeProof) Run(ctx context.Context) (report ComposeReport, err error) {
	report = ComposeReport{Project: p.project.Name, Image: p.runImage(), BreakVolume: p.breakVolume, BreakCommit: p.breakCommit}
	if err := look(interoplab.DockerExecutable); err != nil {
		return report, err
	}
	// Before any step: nothing exists yet, so a refusal here owes no teardown.
	if err := requireCompose(ctx, p.docker, hostGOOS); err != nil {
		return report, err
	}
	if p.image != "" {
		if err := p.requireSuppliedImage(ctx); err != nil {
			return report, err
		}
	}
	p.noticeEarlierVolumes(ctx, &report)

	scratch, err := scratchDir(p.root, "compose-")
	if err != nil {
		return report, err
	}
	overridePath := filepath.Join(scratch, "override.yaml")
	content, err := p.override()
	if err != nil {
		os.RemoveAll(scratch) //nolint:errcheck // nothing was written into it
		return report, err
	}
	if err := os.WriteFile(overridePath, []byte(content), 0o600); err != nil {
		os.RemoveAll(scratch) //nolint:errcheck // the write failed
		return report, err
	}
	p.project.Files = []string{filepath.Join(p.root, "docker", "compose.yaml"), overridePath}

	defer func() {
		p.teardown(context.WithoutCancel(ctx), &report)
		if removeErr := os.RemoveAll(scratch); removeErr != nil {
			report.LeftBehind = append(report.LeftBehind, "scratch directory "+scratch+": "+removeErr.Error())
		}
		if report.Proven && len(report.LeftBehind) > 0 {
			report.Proven = false
			report.Reason = "the journey passed but teardown left objects behind"
		}
	}()

	p.walk(ctx, &report)
	return report, nil
}

// noticeEarlierVolumes names each store volume an interrupted earlier run left
// and the command that removes it, then lets the run carry on. It removes
// nothing: the lab cleanup never touches a volume, and the decision to drop an
// operator's store stays with the person who reads the notice.
func (p *composeProof) noticeEarlierVolumes(ctx context.Context, report *ComposeReport) {
	names, err := p.docker.ComposeProofVolumes(ctx, p.project.Name)
	if err != nil {
		report.EarlierVolumes = append(report.EarlierVolumes, "could not list earlier runs' volumes: "+err.Error())
		p.say("notice: could not list the volumes earlier runs left: " + err.Error() +
			". Check with `docker volume ls --filter label=ze.lab=1`")
		return
	}
	for _, name := range names {
		notice := composeEarlierVolumeNotice(name)
		report.EarlierVolumes = append(report.EarlierVolumes, notice)
		p.say("notice: " + notice)
	}
}

// composeEarlierVolumeNotice says what a leftover volume holds and how to
// remove it.
func composeEarlierVolumeNotice(name string) string {
	return "volume " + name + " is the store of an interrupted earlier run, holding a hashed operator" +
		" credential and an SSH host key; this run does not use it. Fix: `docker volume rm " + name + "`"
}

// requireSuppliedImage refuses a run whose prepared image the daemon does not
// hold. Loading a preview export is the preview guide's step, not this proof's:
// the proof never creates or removes an image it was handed.
func (p *composeProof) requireSuppliedImage(ctx context.Context) error {
	present, err := p.docker.ImagePresent(ctx, p.image)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	return composeImageAbsent(p.image)
}

// composeImageAbsent is the refusal for an image reference the daemon lacks.
func composeImageAbsent(image string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("the Docker daemon holds no image ").Str(image).
		Str(", so the proof has nothing to run. Fix: load the preview export with").
		Str(" `docker load --input <bundle>/ze-preview-<identity>-deployment-linux-<arch>.docker.tar`,").
		Str(" tag the image id its manifest records (`docker tag sha256:<id> ze-local:preview`),").
		Str(" and pass the tag: `./le test deployment docker-compose image ze-local:preview`").String())
}

// composeWalkStep is one named step of the journey.
type composeWalkStep struct {
	name string
	run  func(context.Context, *ComposeReport) (string, error)
}

// steps answers the journey in order. The commit counterfactual inserts its
// overwrite between the two containers, so every step before second-login
// still passes and the persistence assertion is the one that goes red.
func (p *composeProof) steps() []composeWalkStep {
	steps := []composeWalkStep{
		{"build", p.build},
		{"init", p.initStore},
		{"import", p.importConfig},
		{"first-start", p.start},
		{"first-login", p.firstLogin},
		{"edit-commit", p.editAndCommit},
		{"remove-container", p.removeContainer},
	}
	if p.breakCommit {
		steps = append(steps, composeWalkStep{"break-commit", p.overwriteCommit})
	}
	return append(steps,
		composeWalkStep{"second-start", p.start},
		composeWalkStep{"second-login", p.secondLogin},
	)
}

// walk runs every step in order and stops at the first that fails, recording
// it as the verdict.
func (p *composeProof) walk(ctx context.Context, report *ComposeReport) {
	for _, step := range p.steps() {
		detail, err := step.run(ctx, report)
		if err != nil {
			report.Steps = append(report.Steps, ComposeStep{Name: step.name, Detail: err.Error()})
			report.Reason = step.name + ": " + err.Error()
			p.collectLogs(context.WithoutCancel(ctx), report)
			return
		}
		report.Steps = append(report.Steps, ComposeStep{Name: step.name, Passed: true, Detail: detail})
		p.say(step.name + ": " + detail)
	}
	report.Proven = true
}

func (p *composeProof) say(line string) {
	if p.progress == nil {
		return
	}
	io.WriteString(p.progress, "compose: "+line+"\n") //nolint:errcheck // progress output
}

func (p *composeProof) build(ctx context.Context, _ *ComposeReport) (string, error) {
	if p.image != "" {
		return "uses the supplied image " + p.image, nil
	}
	if err := p.docker.ComposeBuild(ctx, p.project, p.progress); err != nil {
		return "", err
	}
	return "built docker/Dockerfile from the checkout as " + p.builtImage(), nil
}

// initStore answers ze init's prompts on standard input: the operator, the
// password, then the defaults the guide tells a user to keep.
func (p *composeProof) initStore(ctx context.Context, _ *ComposeReport) (string, error) {
	answers := composeUser + "\n" + p.password + "\n\n\n\n"
	_, err := p.docker.ComposeRun(ctx, p.project, interoplab.ComposeOneOff{
		Service: composeService, Command: []string{"init"}, Stdin: strings.NewReader(answers),
	})
	if err != nil {
		return "", err
	}
	return "initialized the store for " + composeUser, nil
}

func (p *composeProof) importConfig(ctx context.Context, _ *ComposeReport) (string, error) {
	seed, err := os.Open(filepath.Join(p.root, "docker", "ze.conf"))
	if err != nil {
		return "", err
	}
	defer seed.Close() //nolint:errcheck // read-only
	_, err = p.docker.ComposeRun(ctx, p.project, interoplab.ComposeOneOff{
		Service: composeService, Command: []string{"config", "import", "--yes", "--name", "ze.conf", "-"}, Stdin: seed,
	})
	if err != nil {
		return "", err
	}
	return "imported docker/ze.conf over the seed", nil
}

// overwriteCommit is the commit counterfactual. With no container running it
// imports docker/ze.conf again into the stored ze.conf. An import over an
// existing config replaces it whole as a new active version and changes
// nothing else in the store (cmdImportWithStorage, storage.RestoreConfig), so
// the operator's credential and the SSH host key stay and only the committed
// leaf is gone.
func (p *composeProof) overwriteCommit(ctx context.Context, report *ComposeReport) (string, error) {
	if _, err := p.importConfig(ctx, report); err != nil {
		return "", err
	}
	return "imported docker/ze.conf again over the committed change; the credential and host key stay", nil
}

func (p *composeProof) start(ctx context.Context, report *ComposeReport) (string, error) {
	if err := p.docker.ComposeUp(ctx, p.project); err != nil {
		return "", err
	}
	id, err := p.docker.ComposeContainer(ctx, p.project, composeService)
	if err != nil {
		return "", err
	}
	if report.FirstContainer == "" {
		report.FirstContainer = id
		return "container " + id + " running", nil
	}
	report.SecondContainer = id
	if id == report.FirstContainer {
		return "", errors.New("up answered the removed container " + id + ", not a new one")
	}
	return "new container " + id + " running", nil
}

func (p *composeProof) address(ctx context.Context) (string, error) {
	address, err := p.docker.ComposePublishedPort(ctx, p.project, composeService, interoplab.TCPPort(composeSSHPort))
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

// firstLogin logs in with the initialized password, records the host key, and
// requires the edit to be absent so that its later presence proves the commit.
func (p *composeProof) firstLogin(ctx context.Context, report *ComposeReport) (string, error) {
	address, err := p.address(ctx)
	if err != nil {
		return "", err
	}
	login, err := awaitComposeLogin(ctx, address, composeUser, p.password, "", composeLoginTimeout)
	if err != nil {
		return "", err
	}
	report.HostKey = login.hostKey
	level, present, err := login.runningLeaf(composeEditPath)
	login.close()
	if err != nil {
		return "", err
	}
	if present && level == composeEditValue {
		return "", errors.New("the running configuration already holds " + strings.Join(composeEditPath, " ") +
			" " + composeEditValue + " before the edit, so its presence later would prove nothing")
	}
	return "logged in as " + composeUser + " at " + address + ", host key " + login.hostKey, nil
}

// editAndCommit types the change and `commit now` into the SSH editor, then
// waits for the running configuration to hold it.
func (p *composeProof) editAndCommit(ctx context.Context, report *ComposeReport) (string, error) {
	address, err := p.address(ctx)
	if err != nil {
		return "", err
	}
	line := "set " + strings.Join(composeEditPath, " ") + " " + composeEditValue
	tail, err := editInSSHEditor(ctx, address, composeUser, p.password, report.HostKey,
		[]string{line, "commit now"}, func() (bool, error) {
			login, err := dialCompose(ctx, address, composeUser, p.password, report.HostKey)
			if err != nil {
				return false, err
			}
			defer login.close()
			level, present, err := login.runningLeaf(composeEditPath)
			if err != nil {
				return false, err
			}
			return present && level == composeEditValue, nil
		}, composeApplyTimeout)
	report.EditorTail = tail
	if err != nil {
		return "", err
	}
	return "committed `" + line + "` through the SSH editor; the running configuration holds it", nil
}

// removeContainer is the guide's ordinary `down`: the container goes, the
// named volume stays. A volume that did not stay is the failure itself.
func (p *composeProof) removeContainer(ctx context.Context, _ *ComposeReport) (string, error) {
	if err := p.docker.ComposeDown(ctx, p.project, false); err != nil {
		return "", err
	}
	residue, err := p.docker.ComposeResidue(ctx, p.project)
	if err != nil {
		return "", err
	}
	if len(residue.Containers) > 0 {
		return "", errors.New("down left containers " + strings.Join(residue.Containers, ", "))
	}
	if !p.breakVolume && len(residue.Volumes) == 0 {
		return "", errors.New("down removed the named volume, so nothing could persist")
	}
	return "container removed; volumes kept: " + strings.Join(residue.Volumes, ", "), nil
}

// secondLogin requires the same login, the same host key, a refused wrong
// password, and the committed change, in the new container.
func (p *composeProof) secondLogin(ctx context.Context, report *ComposeReport) (string, error) {
	address, err := p.address(ctx)
	if err != nil {
		return "", err
	}
	login, err := awaitComposeLogin(ctx, address, composeUser, p.password, report.HostKey, composeLoginTimeout)
	if err != nil {
		return "", err
	}
	level, present, err := login.runningLeaf(composeEditPath)
	login.close()
	if err != nil {
		return "", err
	}
	if !present || level != composeEditValue {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("the new container's running configuration holds ").
			Str(strings.Join(composeEditPath, " ")).Str(" = ").Str(strings.TrimSpace(level)).
			Str(", not the committed ").Str(composeEditValue).String())
	}
	if err := refuseWrongPassword(ctx, address, composeUser, p.password+"-wrong", report.HostKey); err != nil {
		return "", err
	}
	return "same login and host key at " + address + "; the committed change is running; a wrong password is refused", nil
}

// collectLogs keeps the daemon's last lines for a failed run.
func (p *composeProof) collectLogs(ctx context.Context, report *ComposeReport) {
	text, err := p.docker.ComposeLogs(ctx, p.project, composeService, LogTailLines)
	if err != nil {
		report.LogTail = append(report.LogTail, "compose logs: "+err.Error())
	}
	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		if line != "" {
			report.LogTail = append(report.LogTail, line)
		}
	}
}

// teardown removes what the run created and nothing else: the project's
// containers, network and volume, then the image a tree build tagged. It then
// lists what the project label still names, which is the report's LeftBehind.
func (p *composeProof) teardown(ctx context.Context, report *ComposeReport) {
	if err := p.docker.ComposeDown(ctx, p.project, true); err != nil {
		report.LeftBehind = append(report.LeftBehind, "compose down: "+err.Error())
	}
	if tag := p.builtImage(); tag != "" {
		if err := p.docker.RemoveBuiltImage(ctx, tag); err != nil {
			report.LeftBehind = append(report.LeftBehind, "image "+tag+": "+err.Error())
		}
	}
	residue, err := p.docker.ComposeResidue(ctx, p.project)
	if err != nil {
		report.LeftBehind = append(report.LeftBehind, "residue listing: "+err.Error())
		return
	}
	for _, id := range residue.Containers {
		report.LeftBehind = append(report.LeftBehind, "container "+id+composeRemovalFix(p.project.Name))
	}
	for _, name := range residue.Volumes {
		report.LeftBehind = append(report.LeftBehind, "volume "+name+". Fix: `docker volume rm "+name+"`")
	}
	for _, id := range residue.Networks {
		report.LeftBehind = append(report.LeftBehind, "network "+id+composeRemovalFix(p.project.Name))
	}
}

func composeRemovalFix(project string) string {
	return ". Fix: `docker compose --project-name " + project + " down --volumes`, or `" + cleanupCommand + "`"
}
