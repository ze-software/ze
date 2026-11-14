// Design: docs/architecture/testing/interop.md -- Docker lab lifecycle and failure contracts
// Related: docker.go -- the closed Docker command grammar these specs render into.
// Related: container.go -- the single-container grammar a Compose project replaces.
// Related: orphans.go -- the owner labels the override puts on every Compose object.
//
// compose.go is the closed `docker compose` grammar a deployment proof needs to
// drive a user's Compose file as the Docker guide does: build, one-off runs,
// up, down, the published port, and what the project still holds. The caller
// names a project and its files, never a compose flag.

package interoplab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// composeVerb is the docker CLI plugin every Compose command runs through.
const composeVerb = "compose"

// dockerVolumeObject is the docker CLI word for a named volume.
const dockerVolumeObject = "volume"

// ComposeProjectPrefix starts every Compose proof project's name, so a volume
// an earlier run left is named by it (ComposeProofVolumes).
const ComposeProjectPrefix = "ze-compose-"

// ownedVolumeName reports whether name is a Compose proof project's volume:
// the prefix, then only the characters a project and volume name hold, so a
// removal command a notice prints never carries anything else.
func ownedVolumeName(name string) bool {
	rest, found := strings.CutPrefix(name, ComposeProjectPrefix)
	if !found {
		return false
	}
	if rest == "" {
		return false
	}
	for _, r := range rest {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !valid {
			return false
		}
	}
	return true
}

// ComposeProofVolumes lists the named volumes Compose proof runs left: a
// volume carrying the lab marker label whose name is a proof project's
// (ownedVolumeName), except those of the project named current. It removes
// nothing: the lab cleanup never touches a volume, so the proof only names
// what it found.
func (d *Docker) ComposeProofVolumes(ctx context.Context, current string) ([]string, error) {
	result, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, dockerVolumeObject, "ls", "--quiet",
		dockerFilterFlag, "label="+labelMarker+"="+labelMarkerValue)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Fields(result.Stdout) {
		if !ownedVolumeName(name) {
			continue
		}
		if strings.HasPrefix(name, current+"_") {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// ComposeProjectLabel is the label Docker Compose puts on every container,
// network and volume it creates for a project. ComposeResidue filters on it.
const ComposeProjectLabel = "com.docker.compose.project"

// composeCommandTimeout bounds a Compose verb that starts or removes objects
// rather than building: up, down and a one-off run of a short command.
const composeCommandTimeout = 3 * time.Minute

// ComposeProject is one Compose project: its name and the files that define it,
// in the order Compose merges them. The name is what makes a run's objects its
// own on a Docker daemon many sessions share, so it is required.
type ComposeProject struct {
	Name  string
	Files []string
}

// arguments renders the project prefix every Compose verb starts with.
func (p ComposeProject) arguments() ([]string, error) {
	if err := validComposeProjectName(p.Name); err != nil {
		return nil, err
	}
	if len(p.Files) == 0 {
		return nil, errors.New("compose project " + p.Name + " names no file")
	}
	argv := []string{DockerExecutable, composeVerb, "--project-name", p.Name}
	for _, file := range p.Files {
		if file == "" {
			return nil, errors.New("compose project " + p.Name + " names an empty file path")
		}
		argv = append(argv, "--file", file)
	}
	return argv, nil
}

// validComposeProjectName refuses a name Compose would refuse or rewrite. Compose
// lowercases and accepts only letters, digits, dashes and underscores, first a
// letter or digit; a name it rewrote would no longer match the label filter.
func validComposeProjectName(name string) error {
	if name == "" {
		return errors.New("compose project name is empty")
	}
	for index, r := range name {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if index > 0 && (r == '-' || r == '_') {
			continue
		}
		return errors.New("compose project name " + strconv.Quote(name) +
			" holds " + strconv.QuoteRune(r) + ": use lowercase letters, digits, '-' and '_', starting with a letter or digit")
	}
	return nil
}

// ComposeOneOff is one `docker compose run --rm -T` of a service: a command the
// image's entrypoint runs once in a fresh container on the project's volumes.
type ComposeOneOff struct {
	Service string
	Command []string
	// Stdin is streamed to the command; nil reads nothing.
	Stdin   io.Reader
	Timeout time.Duration
}

// ComposeVersion answers the Compose plugin's version as it prints it
// (`docker compose version --short`, such as "2.29.1" or "v2.29.1"). A docker
// CLI without the plugin answers an error, never an empty version.
func (d *Docker) ComposeVersion(ctx context.Context) (string, error) {
	result, err := d.commandAs(ctx, dockerCommandTimeout, "docker compose version",
		DockerExecutable, composeVerb, "version", "--short")
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(result.Stdout)
	if version == "" {
		return "", errors.New("docker compose version printed no version")
	}
	return version, nil
}

// ComposeBuild builds every service image of the project, under the machine's
// build budget. progress receives Compose's own build output when non-nil.
func (d *Docker) ComposeBuild(ctx context.Context, project ComposeProject, progress io.Writer) error {
	argv, err := project.arguments()
	if err != nil {
		return err
	}
	argv = append(argv, "build")
	result, err := d.run(ctx, ProcessCommand{Arguments: argv, Timeout: d.buildTimeout, Stdout: progress, Stderr: progress})
	_, err = commandOutcome(result, err, d.buildTimeout, "docker compose build", argv)
	return err
}

// ComposeRun runs one command once in a new container of service, removed when
// the command ends, with no terminal: the Docker guide's `run --rm` steps.
func (d *Docker) ComposeRun(ctx context.Context, project ComposeProject, oneOff ComposeOneOff) (CommandResult, error) {
	argv, err := composeRunArguments(project, oneOff)
	if err != nil {
		return CommandResult{}, err
	}
	timeout := oneOff.Timeout
	if timeout <= 0 {
		timeout = composeCommandTimeout
	}
	result, err := d.run(ctx, ProcessCommand{Arguments: argv, Timeout: timeout, Stdin: oneOff.Stdin})
	result, err = commandOutcome(result, err, timeout, "docker compose run", argv)
	return CommandResult(result), err
}

// composeRunArguments renders a one-off run.
func composeRunArguments(project ComposeProject, oneOff ComposeOneOff) ([]string, error) {
	if oneOff.Service == "" {
		return nil, errors.New("compose run names no service")
	}
	argv, err := project.arguments()
	if err != nil {
		return nil, err
	}
	argv = append(argv, "run", "--rm", "--no-TTY")
	if oneOff.Stdin == nil {
		argv = append(argv, "--interactive=false")
	}
	argv = append(argv, oneOff.Service)
	return append(argv, oneOff.Command...), nil
}

// ComposeUp creates and starts the project's services detached, never building:
// the caller built or supplied the image first, so a missing image is an error
// rather than a build nobody asked for.
func (d *Docker) ComposeUp(ctx context.Context, project ComposeProject) error {
	argv, err := project.arguments()
	if err != nil {
		return err
	}
	argv = append(argv, "up", "--detach", "--no-build")
	_, err = d.commandAs(ctx, composeCommandTimeout, "docker compose up", argv...)
	return err
}

// ComposeDown stops and removes the project's containers and networks. With
// volumes it also removes the project's named volumes, which discards the
// installation; without, the volumes stay, as the guide's ordinary `down` does.
func (d *Docker) ComposeDown(ctx context.Context, project ComposeProject, volumes bool) error {
	argv, err := composeDownArguments(project, volumes)
	if err != nil {
		return err
	}
	_, err = d.commandAs(ctx, composeCommandTimeout, "docker compose down", argv...)
	return err
}

// composeDownArguments renders a down. It never names `--rmi`: an image the
// caller supplied is not the run's to remove, and a built one is removed by tag.
func composeDownArguments(project ComposeProject, volumes bool) ([]string, error) {
	argv, err := project.arguments()
	if err != nil {
		return nil, err
	}
	argv = append(argv, "down", "--remove-orphans")
	if volumes {
		argv = append(argv, "--volumes")
	}
	return argv, nil
}

// ComposeContainer answers the id of the running container of service. A
// service with no container, or more than one, is an error, never an empty id.
func (d *Docker) ComposeContainer(ctx context.Context, project ComposeProject, service string) (string, error) {
	argv, err := project.arguments()
	if err != nil {
		return "", err
	}
	argv = append(argv, "ps", "--quiet", service)
	result, err := d.commandAs(ctx, dockerCommandTimeout, "docker compose ps", argv...)
	if err != nil {
		return "", err
	}
	ids := strings.Fields(result.Stdout)
	if len(ids) != 1 {
		return "", fmt.Errorf("compose project %s holds %d containers of service %s, want one: %q",
			project.Name, len(ids), service, result.Stdout)
	}
	return ids[0], nil
}

// ComposePublishedPort answers the host address Compose published a service's
// container port at. An answer that does not parse is an error naming what
// Compose printed, never port 0.
func (d *Docker) ComposePublishedPort(ctx context.Context, project ComposeProject, service string, port ContainerPort) (netip.AddrPort, error) {
	value, err := port.argument()
	if err != nil {
		return netip.AddrPort{}, err
	}
	argv, err := project.arguments()
	if err != nil {
		return netip.AddrPort{}, err
	}
	number, protocol, _ := strings.Cut(value, "/")
	argv = append(argv, "port", "--protocol", protocol, service, number)
	result, err := d.commandAs(ctx, dockerCommandTimeout, "docker compose port", argv...)
	if err != nil {
		return netip.AddrPort{}, err
	}
	line, _, _ := strings.Cut(strings.TrimSpace(result.Stdout), "\n")
	address, err := netip.ParseAddrPort(strings.TrimSpace(line))
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("docker compose port %s %s answered %q, which is no address and port: %w",
			service, value, result.Stdout, err)
	}
	return address, nil
}

// ComposeLogs answers the last lines a service's containers wrote.
func (d *Docker) ComposeLogs(ctx context.Context, project ComposeProject, service string, lines int) (string, error) {
	argv, err := project.arguments()
	if err != nil {
		return "", err
	}
	argv = append(argv, "logs", "--no-color", "--tail", strconv.Itoa(lines), service)
	result, err := d.commandAs(ctx, dockerLogsTimeout, "docker compose logs", argv...)
	return result.Stdout + result.Stderr, err
}

// ComposeResidue is what the daemon still holds under a project's label.
type ComposeResidue struct {
	Containers []string `json:"containers,omitempty"`
	Volumes    []string `json:"volumes,omitempty"`
	Networks   []string `json:"networks,omitempty"`
}

// Empty reports whether the project left nothing behind.
func (r ComposeResidue) Empty() bool {
	return len(r.Containers) == 0 && len(r.Volumes) == 0 && len(r.Networks) == 0
}

// ComposeResidue lists every container, volume and network that carries the
// project's Compose label. A listing that fails is an error, never "none".
func (d *Docker) ComposeResidue(ctx context.Context, project ComposeProject) (ComposeResidue, error) {
	if err := validComposeProjectName(project.Name); err != nil {
		return ComposeResidue{}, err
	}
	filter := ComposeProjectLabel + "=" + project.Name
	var residue ComposeResidue
	listings := []struct {
		argv []string
		into *[]string
	}{
		{[]string{DockerExecutable, "ps", "--all", "--quiet", dockerFilterFlag, "label=" + filter}, &residue.Containers},
		{[]string{DockerExecutable, "volume", "ls", "--quiet", dockerFilterFlag, "label=" + filter}, &residue.Volumes},
		{[]string{DockerExecutable, "network", "ls", "--quiet", dockerFilterFlag, "label=" + filter}, &residue.Networks},
	}
	for _, listing := range listings {
		result, err := d.command(ctx, dockerCommandTimeout, listing.argv...)
		if err != nil {
			return residue, err
		}
		*listing.into = strings.Fields(result.Stdout)
	}
	return residue, nil
}

// RemoveBuiltImage removes an image tag the caller built under a name only it
// used, together with the image when no other tag names it. Unlike
// ReleaseImage it prunes, so a run that built a whole image leaves no dangling
// copy behind. An absent tag is not an error. The caller MUST own tag.
func (d *Docker) RemoveBuiltImage(ctx context.Context, tag string) error {
	if tag == "" {
		return errors.New("built image tag is empty")
	}
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, dockerImageObject, "rm", tag)
	if err == nil {
		return nil
	}
	if absentAnswer(err, tag) {
		return nil
	}
	return err
}

// ComposeOverride is the second Compose file a proof merges over the user's
// file: the changes a shared Docker daemon forces, and nothing else.
type ComposeOverride struct {
	Service string
	// Image replaces the service's image reference, so a build tags a name only
	// this run uses and a supplied image runs as it is.
	Image string
	// Ports replaces the service's published ports with loopback ports the
	// daemon picks, so no fixed host port collides with another session.
	Ports []ContainerPort
	// DropVolumes removes every volume mount from the service: the counterfactual
	// in which the store lives in each container's own writable layer.
	DropVolumes bool
	// Volume names the project's named volume to label, empty for none. It
	// must be a volume the user's file declares: the override adds labels to
	// that declaration and never declares a volume of its own.
	Volume string
}

// ComposeOverrideFile renders override as a Compose file. The service's
// containers and the project's default network carry this client's owner
// labels, so `./le test integration clean` judges a container or network a
// killed run left as it judges any other lab object. The named volume carries
// them too, but the cleanup removes no volume: a later proof run names a
// leftover one with its removal command (ComposeProofVolumes).
func (d *Docker) ComposeOverrideFile(override ComposeOverride) (string, error) {
	if override.Service == "" {
		return "", errors.New("compose override names no service")
	}
	if override.Image == "" {
		return "", errors.New("compose override names no image")
	}
	var tb textbuf.Buffer
	tb.Str("# Written by le for one proof run; merged over the user's Compose file.\n")
	tb.Str("services:\n  ").Str(override.Service).Str(":\n")
	tb.Str("    image: ").Str(strconv.Quote(override.Image)).Byte('\n')
	tb.Str("    pull_policy: never\n")
	tb.Str("    ports: !override\n")
	for _, port := range override.Ports {
		value, err := port.argument()
		if err != nil {
			return "", err
		}
		tb.Str("      - ").Str(strconv.Quote(publishLoopback + value)).Byte('\n')
	}
	if override.DropVolumes {
		tb.Str("    volumes: !reset []\n")
	}
	writeComposeLabels(&tb, "    ", d.owner.labelPairs(resourceContainer))
	tb.Str("networks:\n  default:\n")
	writeComposeLabels(&tb, "    ", d.owner.labelPairs(resourceNetwork))
	if override.Volume != "" {
		tb.Str("volumes:\n  ").Str(override.Volume).Str(":\n")
		writeComposeLabels(&tb, "    ", d.owner.labelPairs(resourceVolume))
	}
	return tb.String(), nil
}

// writeComposeLabels writes a labels mapping. strconv.Quote's escapes are valid
// in a YAML double-quoted scalar, so a label value cannot break the document.
func writeComposeLabels(tb *textbuf.Buffer, indent string, pairs [][2]string) {
	tb.Str(indent).Str("labels:\n")
	for _, pair := range pairs {
		tb.Str(indent).Str("  ").Str(strconv.Quote(pair[0])).Str(": ").Str(strconv.Quote(pair[1])).Byte('\n')
	}
}
