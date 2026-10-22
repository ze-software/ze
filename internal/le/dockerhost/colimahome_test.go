package dockerhost

import (
	"slices"
	"testing"
)

// TestColimaHomeFollowsColimasResolution walks every branch of Colima
// 0.10.3's configBaseDir with a fake filesystem and checks the root, and the
// `ze` socket Environment names under it.
//
// VALIDATES: review R1-6: COLIMA_HOME (when it exists), then ~/.colima (when
// it exists), then $XDG_CONFIG_HOME/colima, then ~/.colima on macOS, then the
// user config directory; one resolution serves this package and setup's.
// PREVENTS: le naming ~/.colima/ze/docker.sock while Colima serves the
// profile under COLIMA_HOME or XDG_CONFIG_HOME, so the labs refuse a running
// profile or setup recreates one it cannot see.
func TestColimaHomeFollowsColimasResolution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		goos    string
		environ []string
		dirs    []string
		want    string
	}{
		{"colima home exists", "darwin", []string{"COLIMA_HOME=/opt/colima"}, []string{"/opt/colima", "/home/me/.colima"}, "/opt/colima"},
		{"colima home absent", "darwin", []string{"COLIMA_HOME=/opt/colima"}, []string{"/home/me/.colima"}, "/home/me/.colima"},
		{"last colima home wins", "darwin", []string{"COLIMA_HOME=/nowhere", "COLIMA_HOME=/opt/colima"}, []string{"/opt/colima"}, "/opt/colima"},
		{"dot colima beats xdg", "darwin", []string{"XDG_CONFIG_HOME=/home/me/.config"}, []string{"/home/me/.colima"}, "/home/me/.colima"},
		{"xdg without dot colima", "darwin", []string{"XDG_CONFIG_HOME=/home/me/.config"}, nil, "/home/me/.config/colima"},
		{"macOS default", "darwin", nil, nil, "/home/me/.colima"},
		{"linux default", "linux", nil, nil, "/home/me/.config/colima"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exists := func(path string) bool { return slices.Contains(tc.dirs, path) }
			if got := ColimaHome(tc.goos, tc.environ, "/home/me", exists); got != tc.want {
				t.Errorf("ColimaHome = %q, want %q", got, tc.want)
			}
		})
	}

	socket := func(path string) bool { return path == "/home/me/.config/colima/ze/docker.sock" }
	got, err := Environment("darwin", homeMe, []string{"XDG_CONFIG_HOME=/home/me/.config"}, socket)
	if err != nil {
		t.Fatalf("Environment under XDG_CONFIG_HOME refused: %v", err)
	}
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.config/colima/ze/docker.sock") {
		t.Errorf("Environment under XDG_CONFIG_HOME = %q, want the socket under $XDG_CONFIG_HOME/colima", got)
	}
}
