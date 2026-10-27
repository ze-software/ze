// Design: docs/contributing/gh-pages.md -- selective shared-lab controls
// Related: scenarios_showcase.go -- startFixtures records ownership before readiness

package siteterminaldemo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func controlShowcaseLab(args []string, output io.Writer) error {
	if len(args) != 2 {
		return errors.New("showcase-lab control needs a fixture name and an action")
	}
	state := demoState(showcaseLabID)
	environ := scenarioEnv(showcaseLabID, demoPassword)
	for _, fixture := range append(showcaseLabFixtures(), showcaseCustomer()) {
		if fixture.name != args[0] {
			continue
		}
		if control, ok := fixture.controls[args[1]]; ok {
			answer, err := control(state, environ)
			if err != nil {
				return err
			}
			return writeLine(output, strings.TrimSuffix(answer, "\n"))
		}
		if !fixture.restartable {
			return fmt.Errorf("showcase-lab fixture %s does not support %s", fixture.name, args[1])
		}
		switch args[1] {
		case commandStop:
			if err := stopShowcaseFixture(state, fixture.name); err != nil {
				return err
			}
		case commandStart:
			if err := startFixtures(state, filepath.Join(state, showcasePIDs), environ, []labFixture{fixture}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("showcase-lab fixture %s does not support %s", fixture.name, args[1])
		}
		return writeLine(output, fixture.name+" "+args[1]+" complete")
	}
	return fmt.Errorf("showcase-lab has no fixture %q", args[0])
}

func showcaseLinkControls(link string) map[string]func(string, []string) (string, error) {
	controls := make(map[string]func(string, []string) (string, error), 2)
	for _, status := range []string{"down", "up"} {
		controls[status] = func(_ string, _ []string) (string, error) {
			_, err := runCommand("ip", []string{ipLink, ipSet, link, status}, commandOptions{})
			return link + " " + status, err
		}
	}
	return controls
}

// stopShowcaseFixture stops only the PIDs that startFixtures recorded for this
// fixture, waits for exit, and removes them from the aggregate ownership file.
// It MUST finish before that fixture can be started again.
func stopShowcaseFixture(state, name string) error {
	path := filepath.Join(state, name+".pid")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// An absent ownership file is the stopped state; never discover PIDs by name.
		return nil
	}
	if err != nil {
		return err
	}
	owned := strings.Fields(string(data))
	for _, field := range owned {
		pid, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("invalid owned PID in %s: %w", path, err)
		}
		if err := terminateShowcasePID(pid); err != nil {
			return err
		}
	}
	allPath := filepath.Join(state, showcasePIDs)
	all, err := os.ReadFile(allPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	remaining := make([]int, 0)
	for _, field := range strings.Fields(string(all)) {
		remove := false
		for _, stopped := range owned {
			if stopped == field {
				remove = true
				break
			}
		}
		if remove {
			continue
		}
		pid, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("invalid aggregate PID: %w", err)
		}
		remaining = append(remaining, pid)
	}
	if err := writePIDs(allPath, remaining); err != nil {
		return err
	}
	return os.Remove(path)
}

// terminateShowcasePID is used only with a PID returned by startCommand or
// recorded by startFixtures. Callers MUST NOT pass discovered system PIDs.
func terminateShowcasePID(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to stop invalid owned PID %d", pid)
	}
	for _, signal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		if err := syscall.Kill(pid, signal); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}
		for range 50 {
			alive, err := showcaseProcessAlive(pid)
			if err != nil {
				return err
			}
			if !alive {
				return nil
			}
			// sleep(poll-interval): wait until the owned process exits or becomes a zombie.
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("showcase process %d did not exit after SIGKILL", pid)
}

// showcaseProcessAlive reads Linux process state because a dead fixture can
// remain a zombie until the container's init reaps it; kill(0) alone lies then.
func showcaseProcessAlive(pid int) (bool, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, rest, ok := strings.Cut(string(data), ") ")
	if !ok {
		return false, fmt.Errorf("invalid /proc stat for PID %d", pid)
	}
	return !strings.HasPrefix(rest, "Z "), nil
}

// showcasePID reads the shown daemon's own PID, never the first fixture PID.
func showcasePID() (int, error) {
	data, err := os.ReadFile(filepath.Join(demoState(showcaseLabID), "ze.pid"))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	if pid <= 1 {
		return 0, fmt.Errorf("invalid shown Ze PID %d", pid)
	}
	alive, err := showcaseProcessAlive(pid)
	if err != nil {
		return 0, err
	}
	if !alive {
		return 0, fmt.Errorf("shown Ze PID %d is no longer running", pid)
	}
	return pid, nil
}

func requireShowcasePID(expected int) error {
	actual, err := showcasePID()
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("shown Ze restarted: PID %d became %d", expected, actual)
	}
	return nil
}
