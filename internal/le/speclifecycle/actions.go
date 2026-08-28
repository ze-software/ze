// Design: docs/architecture/core-design.md -- native spec lifecycle commands
// Related: session.go -- spec ownership and state paths
// Related: review.go -- independent review artifacts

package speclifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/le/lepath"
)

const commandName = "spec-session"

// Answer is `le spec-session`. Every value follows a closed keyword.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return lifecycleError(err, 2)
	}
	if len(args) == 0 {
		owner, err := newSpecOwner(root)
		if err != nil {
			return lifecycleError(err, 2)
		}
		spec, err := owner.currentSpec()
		if err != nil {
			return lifecycleError(err, 2)
		}
		return currentReport{Spec: spec}, 0
	}
	switch args[0] {
	case "wip":
		if len(args) != 1 {
			return nil, refuseLifecycle(args[1])
		}
		report, err := wip(root, configuredWIPCap())
		if err != nil {
			return lifecycleError(err, 2)
		}
		return report, 0
	case "model":
		return answerModel(root, args[1:])
	case "current", "claim", "release", "state", "review":
		owner, err := newSpecOwner(root)
		if err != nil {
			return lifecycleError(err, 2)
		}
		return answerOwned(owner, args)
	default:
		return nil, refuseLifecycle(args[0])
	}
}

func answerOwned(owner specOwner, args []string) (any, int) {
	switch args[0] {
	case "current":
		if len(args) != 1 {
			return nil, refuseLifecycle(args[1])
		}
		spec, err := owner.currentSpec()
		if err != nil {
			return lifecycleError(err, 2)
		}
		return currentReport{Spec: spec}, 0
	case "claim":
		if len(args) != 3 {
			return nil, refuseLifecycle(firstArgument(args, 1))
		}
		if args[1] != "spec" {
			return nil, refuseLifecycle(args[1])
		}
		report, err := owner.Claim(args[2])
		if err != nil {
			return lifecycleError(err, 2)
		}
		if report.Refused {
			return report, 3
		}
		return report, 0
	case "release":
		if len(args) != 1 {
			return nil, refuseLifecycle(args[1])
		}
		if err := owner.Release(); err != nil {
			return lifecycleError(err, 2)
		}
		return releaseReport{Released: true}, 0
	case "state":
		return answerState(owner, args[1:])
	case "review":
		return answerReview(owner.Root, owner.SessionID, args[1:])
	default:
		return nil, refuseLifecycle(args[0])
	}
}

func answerState(owner specOwner, args []string) (any, int) {
	if len(args) == 1 {
		if args[0] == "current" {
			path, err := owner.StateFile()
			if err != nil {
				return lifecycleError(err, 2)
			}
			return statePathReport{Path: path}, 0
		}
	}
	if len(args) == 3 {
		if args[0] == "latest" {
			if args[1] == "spec" {
				path, err := LatestStateForSpec(owner.Root, args[2])
				if err != nil {
					return lifecycleError(err, 2)
				}
				return statePathReport{Path: path}, 0
			}
		}
	}
	return nil, refuseLifecycle(firstArgument(args, 0))
}

func answerModel(root string, args []string) (any, int) {
	if len(args) == 1 {
		if args[0] != "current" {
			return nil, refuseLifecycle(args[0])
		}
		return answerRunningModel(TranscriptPath(root))
	}
	if len(args) != 3 {
		return nil, refuseLifecycle(firstArgument(args, 0))
	}
	if args[0] != "current" {
		return nil, refuseLifecycle(args[0])
	}
	if args[1] != "transcript" {
		return nil, refuseLifecycle(args[1])
	}
	path, err := readableTranscriptPath(args[2])
	if err != nil {
		return lifecycleError(err, 2)
	}
	return answerRunningModel(path)
}

func answerRunningModel(path string) (any, int) {
	model := RunningModel(path)
	report := modelReport{Transcript: path, Model: model, ReviewTier: IsReviewTier(model), Readable: model != ""}
	if model == "" {
		return report, 1
	}
	return report, 0
}

func readableTranscriptPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("model transcript needs a non-empty path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("model transcript path must be absolute: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read model transcript %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("model transcript is not a regular file: %s", path)
	}
	return path, nil
}

func answerReview(root, sid string, args []string) (any, int) {
	if len(args) == 0 {
		return nil, refuseLifecycle("review")
	}
	switch args[0] {
	case "hash":
		files, err := repeatedValues(args[1:], "file")
		if err != nil {
			return lifecycleError(err, 2)
		}
		if len(files) == 0 {
			return lifecycleError(errors.New("review hash needs file <path>"), 2)
		}
		report := reviewHashes{Files: make([]ReviewedFile, 0, len(files))}
		for _, path := range uniqueSorted(files) {
			hash, err := reviewFileHash(root, path)
			if err != nil {
				return lifecycleError(err, 2)
			}
			report.Files = append(report.Files, ReviewedFile{Path: path, Hash: hash})
		}
		return report, 0
	case "record":
		request, err := parseReviewRecord(args[1:])
		if err != nil {
			return lifecycleError(err, 2)
		}
		request.SessionID = sid
		artifact, err := recordReview(root, request)
		if err != nil {
			return lifecycleError(err, 2)
		}
		for _, warning := range artifact.Warnings {
			fmt.Fprintln(os.Stderr, warning) //nolint:errcheck // CLI output
		}
		return artifact, 0
	case "check":
		spec, files, err := parseReviewCheck(args[1:])
		if err != nil {
			return lifecycleError(err, 2)
		}
		check, err := CheckReview(root, spec, sid, files)
		if err != nil {
			return lifecycleError(err, 2)
		}
		for _, warning := range check.Warnings {
			fmt.Fprintln(os.Stderr, warning) //nolint:errcheck // CLI output
		}
		if check.Blocked {
			return check, 3
		}
		return check, 0
	default:
		return nil, refuseLifecycle(args[0])
	}
}

func parseReviewRecord(args []string) (reviewRecord, error) {
	var request reviewRecord
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return reviewRecord{}, fmt.Errorf("review record keyword %q needs a value", args[index])
		}
		key, value := args[index], args[index+1]
		switch key {
		case "spec":
			request.Spec = value
		case "verdict":
			request.Verdict = value
		case "rounds":
			rounds, err := strconv.Atoi(value)
			if err != nil {
				return reviewRecord{}, fmt.Errorf("review rounds %q is not an integer", value)
			}
			request.Rounds = rounds
		case "file":
			request.Files = append(request.Files, value)
		case "reviewers":
			request.Reviewers = value
		case "findings-file":
			request.FindingsFile = value
		case "rounds-reason":
			request.RoundsReason = value
		case "owner-authorised":
			request.OwnerAuthorised = value
		case "model-override":
			request.ModelOverride = value
		default:
			return reviewRecord{}, fmt.Errorf("review record does not accept keyword %q", key)
		}
	}
	if request.Spec == "" {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if request.Verdict == "" {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if request.Rounds == 0 {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	if len(request.Files) == 0 {
		return reviewRecord{}, errors.New("review record needs spec, verdict, rounds, and at least one file")
	}
	return request, nil
}

func parseReviewCheck(args []string) (string, []string, error) {
	var spec string
	var files []string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return "", nil, fmt.Errorf("review check keyword %q needs a value", args[index])
		}
		switch args[index] {
		case "spec":
			spec = args[index+1]
		case "file":
			files = append(files, args[index+1])
		default:
			return "", nil, fmt.Errorf("review check does not accept keyword %q", args[index])
		}
	}
	if spec == "" {
		return "", nil, errors.New("review check needs spec <stem>")
	}
	return spec, files, nil
}

func repeatedValues(args []string, keyword string) ([]string, error) {
	var values []string
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return nil, fmt.Errorf("expected %s <value>", keyword)
		}
		if args[index] != keyword {
			return nil, fmt.Errorf("expected %s <value>", keyword)
		}
		values = append(values, args[index+1])
	}
	return values, nil
}

func lifecycleError(err error, code int) (any, int) {
	fmt.Fprintf(os.Stderr, "error: spec-session: %v\n", err) //nolint:errcheck // CLI output
	return nil, code
}

func firstArgument(args []string, index int) string {
	if index < 0 {
		return ""
	}
	if index >= len(args) {
		return ""
	}
	return args[index]
}

func refuseLifecycle(got string) int {
	fmt.Fprintf(os.Stderr, "usage: le spec-session {claim spec <spec>|current|release|wip|state current|state latest spec <stem>|model current [transcript <absolute-path>]|review <hash|record|check>} (got %q)\n", got) //nolint:errcheck // CLI output
	return 2
}
