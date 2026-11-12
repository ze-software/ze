// Design: docs/guide/netlab.md -- run upstream lab tools on the Docker host.
package testnetlab

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/linuxle"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

type labAnswer struct {
	Directory string `json:"directory"`
	Clean     bool   `json:"clean"`
}

func packageHere(args leaction.Arguments) (any, int) {
	directory := args.One("directory")
	if directory == "" {
		leaction.ReportError(opererr.New("netlab package needs a destination directory",
			"Export creates a new self-contained lab directory. Supply an unused destination path.",
			netlabPackageCommand("/absolute/path/to/new-lab")))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "netlab package cannot find the Ze checkout",
			"The topology and templates cannot be located. Change to the Ze checkout containing go.mod and feature-gates.txt before running this command.",
			netlabPackageCommand(directory)))
		return nil, 1
	}
	if err := newChecker(root).Package(directory); err != nil {
		leaction.ReportError(opererr.Wrap(err, "netlab package could not export "+directory,
			"No complete reusable lab was exported. Resolve the reported input or filesystem failure before running this command; choose a new destination if this directory already exists.",
			netlabPackageCommand(directory)))
		return nil, 1
	}
	return labAnswer{Directory: directory, Clean: true}, 0
}

func runLiveHere(args leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if args.One("runtime") == "host" {
		directory := args.One("directory")
		if directory == "" {
			leaction.ReportError(opererr.New("netlab runtime host needs a packaged lab directory",
				"Host mode runs inside the prepared tool environment with Docker, netlab and containerlab available, a staged ze binary, and a reachable Docker daemon running Ze's kernel. Supply that packaged lab's path.",
				netlabHostCommand("/absolute/path/to/packaged-lab")))
			return nil, 2
		}
		err := interoplab.DockerKernel(filepath.Join(directory, "ze"))(ctx, interoplab.NewDocker())
		if err == nil {
			err = newChecker("").RunLive(ctx, directory)
		}
		if err != nil {
			leaction.ReportError(opererr.Wrap(err, "netlab host runtime could not finish the proof in "+directory,
				"Host mode needs this package's staged ze, Docker, netlab and containerlab, and access to a Docker daemon running Ze's kernel. Inspect the retained artifacts and resolve the reported prerequisite, protocol or cleanup failure before running this command.",
				netlabHostCommand(directory)))
			return nil, 1
		}
		return labAnswer{Directory: directory, Clean: true}, 0
	}
	if args.Has("directory") {
		leaction.ReportError(opererr.New("netlab run directory requires runtime host",
			"Container mode creates its own lab directory. Use host mode for the named package inside the prepared tool environment on the Ze Docker host.",
			netlabHostCommand(args.One("directory"))))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "netlab run cannot find the Ze checkout",
			"The lab sources and build inputs cannot be located. Change to the Ze checkout containing go.mod and feature-gates.txt before running this command.",
			"./le test netlab run"))
		return nil, 1
	}
	directory, err := runContainerLab(ctx, root)
	if err != nil {
		problem := "netlab live proof could not finish"
		if directory != "" {
			problem += "; retained artifacts: " + directory
		}
		leaction.ReportError(opererr.Wrap(err, problem,
			"The run needs the checkout's Go toolchain, Docker and a reachable daemon running Ze's kernel; its tool image supplies netlab and containerlab. Inspect the retained artifacts when present and resolve the reported build, prerequisite, protocol or cleanup failure before running this command.",
			"./le test netlab run"))
		return nil, 1
	}
	return labAnswer{Directory: directory, Clean: true}, 0
}

func runContainerLab(ctx context.Context, root string) (directory string, err error) {
	session, err := lepath.ResolveSession(root, true)
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(filepath.Join(root, session.Scratch), "netlab-live-")
	if err != nil {
		return "", err
	}
	directory = filepath.Join(work, "lab")
	checker := newChecker(root)
	if err := checker.Package(directory); err != nil {
		return directory, err
	}
	// The generated name isolates node/container names between simultaneous
	// jobs. The package is retained with snapshots and rendered configuration.
	topology := filepath.Join(directory, "topology.yml")
	file, err := os.OpenFile(topology, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return directory, err
	}
	_, writeErr := fmt.Fprintf(file, "\nname: %s\n", filepath.Base(work))
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return directory, err
	}
	docker := interoplab.NewDocker()
	leRelative, err := filepath.Rel(root, filepath.Join(directory, linuxle.Name))
	if err != nil {
		return directory, err
	}
	zeRelative, err := filepath.Rel(root, filepath.Join(directory, "ze"))
	if err != nil {
		return directory, err
	}
	binaries := []interoplab.LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: zeRelative},
		{Name: linuxle.Name, Base: linuxle.Base, Output: leRelative, StartProbe: linuxle.StartProbe()},
	}
	if err := interoplab.StageBinaries(root, false, binaries...)(ctx, docker); err != nil {
		return directory, err
	}
	if err := interoplab.DockerKernel(filepath.Join(directory, "ze"))(ctx, docker); err != nil {
		return directory, err
	}
	nodeImage, err := buildLabNode(ctx, docker, checker, root, directory)
	if err != nil {
		return directory, err
	}
	releaseImage := func(image interoplab.ImageResult) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		err = errors.Join(err, docker.ReleaseImage(cleanup, image))
	}
	defer releaseImage(nodeImage)
	image, err := docker.Build(ctx, toolImageBuild(directory))
	if err != nil {
		return directory, err
	}
	defer releaseImage(image)
	if err := checkNodeImageBinding(ctx, docker, nodeImage); err != nil {
		return directory, err
	}
	mounts := []interoplab.Mount{{Source: directory, Target: directory}}
	for _, path := range []string{"/var/run/docker.sock", "/var/run/netns", "/etc/hosts", "/var/lib/docker/containers"} {
		mounts = append(mounts, interoplab.Mount{Source: path, Target: path})
	}
	_, err = docker.RunOneShot(ctx, interoplab.OneShotContainer{
		Image: image.Reference, Network: "host", Mounts: mounts,
		Arguments: []string{"--privileged", "--pid=host", "--workdir", directory},
		Command:   []string{"/usr/local/bin/le", "test", "netlab", "run", "runtime", "host", "directory", directory},
		Timeout:   20 * time.Minute, Stdout: os.Stdout, Stderr: os.Stderr,
	})
	if err != nil {
		// A canceled docker client can remove the tool container before
		// its signal handler finishes. Use its retained snapshot to finish
		// teardown from a fresh, bounded tool container on the same host.
		_, snapshotErr := os.Stat(filepath.Join(directory, "netlab.snapshot.yml"))
		if snapshotErr == nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
			_, cleanupErr := docker.RunOneShot(cleanup, interoplab.OneShotContainer{
				Image: image.Reference, Network: "host", Mounts: mounts,
				Arguments: []string{"--privileged", "--pid=host", "--workdir", directory},
				Command:   []string{"netlab", "down", "--cleanup"},
				Timeout:   2 * time.Minute, Stdout: os.Stdout, Stderr: os.Stderr,
			})
			cancel()
			err = errors.Join(err, cleanupErr)
		} else if !errors.Is(snapshotErr, os.ErrNotExist) {
			err = errors.Join(err, snapshotErr)
		}
		return directory, fmt.Errorf("live lab (artifacts %s): %w", directory, err)
	}
	return directory, nil
}

func netlabPackageCommand(directory string) string {
	return "./le test netlab package directory " + interoplab.ShellQuoted(directory)
}

func netlabHostCommand(directory string) string {
	return "./le test netlab run runtime host directory " + interoplab.ShellQuoted(directory)
}
