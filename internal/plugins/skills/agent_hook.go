// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent_skill.go — the plan whose hook items these functions act on
//
// Every call that reads or writes a harness's settings file is behind the
// three functions of this file, so the stub commands never open one. This
// build installs no session-start hook: each function leaves the file as it
// is and answers the hook item refused, with the reason and the guide the
// stub already points the agent at.

package skills

// createHook is create agent skill's hook item for the settings file at file,
// an absolute path. This build answers it refused and opens no file.
func createHook(file string) outcome {
	return hookNotBuilt(file)
}

// inspectHook is show agent skill's hook item for the settings file at file.
// This build answers it refused and opens no file.
func inspectHook(file string) outcome {
	return hookNotBuilt(file)
}

// deleteHook is delete agent skill's hook item for the settings file at file.
// This build answers it refused and opens no file.
func deleteHook(file string) outcome {
	return hookNotBuilt(file)
}

// hookNotBuilt is the hook item of a build that carries no settings file
// code: refused, with the file left untouched.
func hookNotBuilt(file string) outcome {
	return outcome{state: stateRefused, path: file, note: noteLines(
		"Ze does not set up the session-start hook yet, so it did not read or change this file.",
		"The ze skill still sends the agent to the guide this ze carries. To read that guide, run:",
		"ze show skill ze")}
}
