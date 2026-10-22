package dockerhost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	labhost "github.com/ze-software/ze/internal/le/dockerhost"
)

// recordingColima is a colimaHost whose colima records every argv and
// plays a profile that exists once started and runs until stopped.
type recordingColima struct {
	argvs   [][]string
	profile profileState
}

func (r *recordingColima) host(t *testing.T, recordPath string) colimaHost {
	t.Helper()
	return colimaHost{
		run: func(_ context.Context, argv []string) error {
			r.argvs = append(r.argvs, argv)
			switch argv[1] {
			case "start":
				r.profile = profileState{exists: true, running: true}
			case "stop":
				r.profile.running = false
			case "delete":
				r.profile = profileState{}
			}
			return nil
		},
		state: func(context.Context) (profileState, error) {
			r.argvs = append(r.argvs, colimaArgv("status"))
			return r.profile, nil
		},
		recordPath: recordPath,
		progress:   &bytes.Buffer{},
	}
}

// verbs answers the colima verbs run, status queries excepted.
func (r *recordingColima) verbs() []string {
	var verbs []string
	for _, argv := range r.argvs {
		if argv[1] != "status" {
			verbs = append(verbs, argv[1])
		}
	}
	return verbs
}

// VALIDATES: wiring row 2 and spec AC-7: the first start starts the profile
// and records K1; with K2 and the profile running, start runs stop, delete
// --force, then start with the new image and records K2; a third start with
// K2 running starts nothing.
// PREVENTS: Colima keeping the old disk and silently ignoring a rebuilt image (R-5).
func TestDockerHostStartRecreatesProfileOnNewImage(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), colimaRecordFileName)
	recorder := &recordingColima{}
	host := recorder.host(t, recordPath)
	ctx := context.Background()

	first, err := host.start(ctx, builtImage{Path: "/cache/k1/docker-host.img", Key: "k1"}, "arm64")
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	if first.Action != startActionStart || !slices.Equal(recorder.verbs(), []string{"start"}) {
		t.Fatalf("first start did %q running %v, want one start", first.Action, recorder.verbs())
	}
	if key, image, err := readColimaRecord(recordPath); err != nil || key != "k1" || image != "/cache/k1/docker-host.img" {
		t.Fatalf("record after the first start: %q %q %v", key, image, err)
	}

	recorder.argvs = nil
	second, err := host.start(ctx, builtImage{Path: "/cache/k2/docker-host.img", Key: "k2"}, "arm64")
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	if second.Action != startActionRecreate || !slices.Equal(recorder.verbs(), []string{"stop", "delete", "start"}) {
		t.Fatalf("second start did %q running %v, want stop, delete, start", second.Action, recorder.verbs())
	}
	var deleteArgv, startArgv []string
	for _, argv := range recorder.argvs {
		switch argv[1] {
		case "delete":
			deleteArgv = argv
		case "start":
			startArgv = argv
		}
	}
	if !slices.Contains(deleteArgv, "--force") {
		t.Errorf("delete argv %v lacks --force", deleteArgv)
	}
	start := strings.Join(startArgv, " ")
	for _, want := range []string{"--disk-image /cache/k2/docker-host.img", "--force-disk-image", "--vm-type vz", "--arch aarch64", "--runtime docker", "--vz-rosetta", "--activate=false"} {
		if !strings.Contains(start, want) {
			t.Errorf("start argv %q lacks %q", start, want)
		}
	}
	if key, _, err := readColimaRecord(recordPath); err != nil || key != "k2" {
		t.Fatalf("record after the second start: %q %v", key, err)
	}

	recorder.argvs = nil
	third, err := host.start(ctx, builtImage{Path: "/cache/k2/docker-host.img", Key: "k2"}, "arm64")
	if err != nil {
		t.Fatalf("third start: %v", err)
	}
	if third.Action != startActionNone || len(recorder.verbs()) != 0 {
		t.Fatalf("third start did %q running %v, want nothing", third.Action, recorder.verbs())
	}
}

// VALIDATES: wiring row 3, spec AC-7 and R-6: every colima argv start, stop
// and the state query produce carries `--profile ze` right after the verb,
// and none names Colima's default profile.
// PREVENTS: setup starting, stopping or deleting the owner's default profile.
func TestDockerHostNeverNamesTheDefaultProfile(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), colimaRecordFileName)
	recorder := &recordingColima{}
	host := recorder.host(t, recordPath)
	ctx := context.Background()
	for _, key := range []string{"k1", "k2", "k2"} {
		if _, err := host.start(ctx, builtImage{Path: "/cache/" + key, Key: key}, "amd64"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := host.stop(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := host.stop(ctx); err != nil {
		t.Fatal(err)
	}
	if len(recorder.argvs) < 6 {
		t.Fatalf("only %d argvs recorded", len(recorder.argvs))
	}
	for _, argv := range recorder.argvs {
		if argv[0] != "colima" || len(argv) < 4 || argv[2] != "--profile" || argv[3] != labhost.ZeProfile {
			t.Errorf("argv %v does not carry --profile ze after its verb", argv)
		}
		if slices.Contains(argv, "default") {
			t.Errorf("argv %v names the default profile", argv)
		}
		if slices.Contains(argv, "--vz-rosetta") {
			t.Errorf("argv %v asks Rosetta for an amd64 image", argv)
		}
	}
}

// VALIDATES: spec R-10: a second setup run while one holds the lock refuses
// at once, naming the holder's pid and the lock file; once released the lock
// is taken again.
// PREVENTS: two sessions deleting the profile under each other's start.
func TestDockerHostStartTakesTheLock(t *testing.T) {
	dir := t.TempDir()
	held, err := takeLock(dir)
	if err != nil {
		t.Fatalf("first takeLock: %v", err)
	}
	_, err = takeLock(dir)
	if err == nil {
		t.Fatal("a second takeLock succeeded while the first holds the lock")
	}
	for _, want := range []string{"pid " + strconv.Itoa(os.Getpid()), filepath.Join(dir, lockFileName), "wait for that run"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
	if err := held.release(); err != nil {
		t.Fatal(err)
	}
	again, err := takeLock(dir)
	if err != nil {
		t.Fatalf("takeLock after release: %v", err)
	}
	if err := again.release(); err != nil {
		t.Fatal(err)
	}
}
