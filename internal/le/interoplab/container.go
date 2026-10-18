// Design: docs/architecture/testing/interop.md -- Docker lab lifecycle and failure contracts
// Related: docker.go -- the closed Docker command grammar these specs render into.
// Related: process.go -- the bounded host process boundary.

package interoplab

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"time"
)

// DetachedContainer is a long-running container a caller outside a lab Suite
// drives by exec: a deployment proof's peer host, a perf DUT or its sender.
// Every field is typed, so the closed grammar stays here: a caller names what
// the container needs, never a docker flag.
type DetachedContainer struct {
	Name  string
	Image string
	// Platform selects the image variant (`linux/amd64`); empty takes the
	// daemon's own.
	Platform string
	// Network and Address attach the container to a user network at a fixed
	// address. Both are empty for the default bridge; an Address needs a
	// Network.
	Network string
	Address netip.Addr
	// Capabilities are added one by one; the container is never privileged.
	Capabilities []string
	// Devices are host device nodes passed through unchanged, such as
	// /dev/net/tun or /dev/ppp.
	Devices          []string
	Mounts           []Mount
	Environment      []EnvironmentVariable
	WorkingDirectory string
	// Entrypoint replaces the image's entry point; empty keeps it.
	Entrypoint string
	Command    []string
}

// ContainerExec is one command run inside a running container with its streams
// attached: a daemon a proof watches, a package install, a query whose output
// is the evidence.
type ContainerExec struct {
	Container   string
	Command     []string
	Environment []EnvironmentVariable
	// Interactive keeps the command's standard input open (`--interactive`).
	// A nil Stdin then reads nothing.
	Interactive bool
	Stdin       io.Reader
	// A nil Stdout or Stderr is captured into the result (ProcessCommand).
	Stdout io.Writer
	Stderr io.Writer
	// Timeout bounds the client. A canceled or expired exec kills the docker
	// client only: the command inside the container lives until it exits or
	// its container is removed.
	Timeout time.Duration
}

// RunDetached starts one labeled detached container. The owner labels are what
// let a later start or `./le test integration cleanup` remove it when this
// process dies before its own RemoveContainer runs.
func (d *Docker) RunDetached(ctx context.Context, spec DetachedContainer) error {
	argv, err := d.detachedArguments(spec)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, dockerRunTimeout, argv...)
	return err
}

// detachedArguments renders spec in a fixed order: the run verb, the name,
// the owner labels, then the container's own needs, then the image and its
// command.
func (d *Docker) detachedArguments(spec DetachedContainer) ([]string, error) {
	if spec.Name == "" {
		return nil, errors.New("detached container name is empty")
	}
	if spec.Image == "" {
		return nil, errors.New("detached container image is empty")
	}
	if spec.Address.IsValid() && spec.Network == "" {
		return nil, errors.New("detached container address needs a network")
	}
	argv := []string{DockerExecutable, "run", "-d", "--name", spec.Name}
	argv = append(argv, d.owner.labelArguments()...)
	if spec.Network != "" {
		argv = append(argv, "--network", spec.Network)
	}
	if spec.Address.IsValid() {
		argv = append(argv, "--ip", spec.Address.String())
	}
	if spec.Platform != "" {
		argv = append(argv, "--platform", spec.Platform)
	}
	for _, capability := range spec.Capabilities {
		argv = append(argv, "--cap-add", capability)
	}
	for _, device := range spec.Devices {
		argv = append(argv, "--device", device)
	}
	for _, mount := range spec.Mounts {
		value, err := mountArgument(mount)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-v", value)
	}
	for _, variable := range spec.Environment {
		value, err := environmentArgument(variable)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "-e", value)
	}
	if spec.WorkingDirectory != "" {
		argv = append(argv, "-w", spec.WorkingDirectory)
	}
	if spec.Entrypoint != "" {
		argv = append(argv, "--entrypoint", spec.Entrypoint)
	}
	argv = append(argv, spec.Image)
	return append(argv, spec.Command...), nil
}

// ExecAttached runs one command in a running container under its own bound,
// with the streams spec attaches. A non-zero exit returns an error carrying
// the result, as Exec does.
func (d *Docker) ExecAttached(ctx context.Context, spec ContainerExec) (CommandResult, error) {
	if spec.Container == "" {
		return CommandResult{}, errors.New("exec container name is empty")
	}
	if len(spec.Command) == 0 {
		return CommandResult{}, errors.New("exec command is empty")
	}
	if spec.Timeout <= 0 {
		return CommandResult{}, errors.New("exec timeout must be positive")
	}
	prefix := []string{DockerExecutable, dockerExecVerb}
	if spec.Interactive {
		prefix = append(prefix, "--interactive")
	}
	argv, err := dockerExecArguments(prefix, spec.Container, spec.Command, spec.Environment)
	if err != nil {
		return CommandResult{}, err
	}
	result, err := d.runner.Run(ctx, ProcessCommand{
		Arguments: argv, Timeout: spec.Timeout, Environment: d.environ,
		Stdin: spec.Stdin, Stdout: spec.Stdout, Stderr: spec.Stderr,
	})
	if err != nil {
		return CommandResult(result), d.oneShotError(argv, spec.Timeout, result, err)
	}
	if result.ExitCode != 0 {
		return CommandResult(result), d.oneShotError(argv, spec.Timeout, result, nil)
	}
	return CommandResult(result), nil
}

// ImagePresent answers whether the daemon holds image. An inspect that fails
// for any reason but absence is an error, never "absent", so a daemon that
// does not answer is not mistaken for a missing image that a pull would fix.
func (d *Docker) ImagePresent(ctx context.Context, image string) (bool, error) {
	if image == "" {
		return false, errors.New("image reference is empty")
	}
	return d.resourcePresent(ctx, resourceImageTag, image)
}
