package interoplab

import (
	"strings"
	"testing"
)

// ownedFlagArguments is every spelling of a free-form docker argument that
// would replace the client's container name or forge an owner label.
var ownedFlagArguments = [][]string{
	{"--name", "other"},
	{"--name=other"},
	{"--label", labelRun + "=forged"},
	{"--label=" + labelRun + "=forged"},
	{"-l", labelPID + "=1"},
	{"-l=" + labelPID + "=1"},
	{"-l" + labelPID + "=1"},
	{"-dl", labelRun + "=forged"},
	{"-dl" + labelRun + "=forged"},
	{"-dl=" + labelRun + "=forged"},
	{"-itl", labelPID + "=1"},
	{"-dPqtil" + labelMarker + "=1"},
	{"--label", labelMarker + "=1"},
	{"--label-file", "/tmp/labels"},
}

// VALIDATES: PeerConfig.Arguments and OneShotContainer.Arguments refuse
// --name and a ze.lab label in every spelling, with a BUG-class error naming
// the field, before docker runs; an unrelated label still passes.
// PREVENTS: a container cancel removal cannot find by name (the 4f054354a3
// class), and a forged owner label overriding the client's own.
func TestFreeArgumentsRefuseNameAndOwnerLabels(t *testing.T) {
	for _, arguments := range ownedFlagArguments {
		t.Run(strings.Join(arguments, " "), func(t *testing.T) {
			runner := &recordingRunner{}
			docker := newDocker(runner)
			peer := PeerConfig{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3, Arguments: arguments}
			err := docker.runContainer(t.Context(), labNetwork(), peer)
			if err == nil || !strings.Contains(err.Error(), "BUG") || !strings.Contains(err.Error(), "PeerConfig.Arguments") {
				t.Errorf("runContainer error = %v, want a BUG naming PeerConfig.Arguments", err)
			}
			_, err = docker.RunOneShot(t.Context(), OneShotContainer{Image: "probe", Timeout: 1, Arguments: arguments})
			if err == nil || !strings.Contains(err.Error(), "BUG") || !strings.Contains(err.Error(), "OneShotContainer.Arguments") {
				t.Errorf("RunOneShot error = %v, want a BUG naming OneShotContainer.Arguments", err)
			}
			if len(runner.commands) != 0 {
				t.Errorf("docker ran despite the refusal:\n%s", recordedCommands(runner.commands))
			}
		})
	}
	for _, arguments := range freeFlagArguments {
		runner := &recordingRunner{}
		peer := PeerConfig{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3, Arguments: arguments}
		if err := newDocker(runner).runContainer(t.Context(), labNetwork(), peer); err != nil {
			t.Errorf("%q was refused: %v", arguments, err)
		}
	}
}

// freeFlagArguments sets no name and no owner label, though an l appears in a
// short-flag cluster: there it is the value of the value-taking flag before it,
// as docker's flag parser reads a cluster.
var freeFlagArguments = [][]string{
	{"--label", "org.example=1", "--network-alias", "named"},
	{"-dl", "org.example=1"},
	{"-el" + labelRun + "=1"},
	{"-it"},
}
