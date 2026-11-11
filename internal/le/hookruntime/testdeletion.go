// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: bash.go -- bashTestDeletion, the guard that judges these arguments
// Related: shellline.go -- lineCommands, the splitter this reads
package hookruntime

import "slices"

// plainRmArguments answers the arguments of every rm on a line whose deletes
// the hook can see whole, and false for any other line, which keeps the
// line-wide judgment of bashTestDeletion.
//
// Such a line has the shell syntax the hook models (modeledSyntax), parses
// whole, holds no command substitution, and holds two kinds of command only:
//   - an `rm` typed as the command word, with no word the shell expands, so
//     its arguments are the paths it deletes;
//   - a reader its judge admits (readerCommand) that writes no file, is not
//     `cd`, which changes what a relative rm argument names, and names no
//     delete word, so `grep -n rm x_test.go` stays line-wide.
//
// Every other way to reach a delete (`git rm`, `find -delete`, xargs, an rm
// after eval, sudo, a loop keyword or an assignment, a variable, a glob, a
// subshell or a group) fails one of these and keeps the line-wide judgment.
func plainRmArguments(command string) ([]string, bool) {
	if !modeledSyntax(command) {
		return nil, false
	}
	line, parsed := lineCommands(command)
	if !parsed {
		return nil, false
	}
	if line.substitutes {
		return nil, false
	}
	arguments := make([]string, 0, 8)
	for index, words := range line.commands {
		if words[0] == "rm" {
			if slices.Contains(line.expands[index], true) {
				return nil, false
			}
			arguments = append(arguments, words[1:]...)
			continue
		}
		if !plainReader(line, index) {
			return nil, false
		}
	}
	return arguments, true
}

// plainReader reports whether the command at index is a reader that leaves
// the rm arguments on its line meaning what they say: a readerCommand that
// writes no file, is not cd, and names no delete word.
func plainReader(line shellLine, index int) bool {
	words := line.commands[index]
	if words[0] == "cd" {
		return false
	}
	if !readerCommand(words, line.expands[index]) {
		return false
	}
	if line.writesFile(index) {
		return false
	}
	return !runsDeletion(words)
}
