// Design: docs/architecture/core-design.md -- registered in-process verification dispatch
//
// Package verifydispatch connects verifyworktree to le's local-data registry.
// It owns no command table: verify.Identity names the tool and exact arguments.
package verifydispatch

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/verify"
)

var rootOverrideMu sync.Mutex


// RunGate runs one registered le tool inside the requested checkout root.
func RunGate(ctx context.Context, root string, identity verify.Identity) verify.GateResult {
	return dispatch(ctx, root, identity, leroot.Owns, lookupTool, lepath.Root)
}

func lookupTool(name string) registry.LocalDataHandler {
	words := [2]string{"le", name}
	handler, trailing := registry.LookupLocalData(words[:])
	if len(trailing) != 0 {
		return nil
	}
	return handler
}

func dispatch(
	ctx context.Context,
	root string,
	identity verify.Identity,
	owns func(string) bool,
	lookup func(string) registry.LocalDataHandler,
	resolveRoot func() (string, error),
) (result verify.GateResult) {
	identity.Args = slices.Clone(identity.Args)
	var text textbuf.Buffer
	result.Identity = identity
	if err := ctx.Err(); err != nil {
		return refused(result, "interrupted", verify.Interrupted, err.Error())
	}
	if strings.TrimSpace(root) == "" {
		return refused(result, "root-missing", 2, "gate runner received no checkout root")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("resolve gate root: ", err))
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("open gate root: ", err))
	}
	if !info.IsDir() {
		return refused(result, "root-missing", 2, "gate root is not a directory")
	}

	rootOverrideMu.Lock()
	defer rootOverrideMu.Unlock()
	if err := ctx.Err(); err != nil {
		return refused(result, "interrupted", verify.Interrupted, err.Error())
	}

	previousRoot := env.Get(lepath.RootKey)
	if err := env.Set(lepath.RootKey, absoluteRoot); err != nil {
		return refused(result, "root-override", 2, messageWithError("set gate root: ", err))
	}
	defer func() {
		if err := env.Set(lepath.RootKey, previousRoot); err != nil {
			result = refused(result, "root-restore", 2, messageWithError("restore gate root: ", err))
		}
	}()

	resolvedRoot, err := resolveRoot()
	if err != nil {
		return refused(result, "root-mismatch", 2, messageWithError("resolve overridden gate root: ", err))
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return refused(result, "root-mismatch", 2, messageWithError("normalize overridden gate root: ", err))
	}
	if filepath.Clean(resolvedRoot) != filepath.Clean(absoluteRoot) {
		message := text.Reset().Str("gate root resolved to ").Quoted(resolvedRoot).
			Str(", want ").Quoted(absoluteRoot).String()
		return refused(result, "root-mismatch", 2, message)
	}
	if !owns(identity.Command) {
		return refused(result, "unowned-root", 2,
			text.Reset().Str("le does not own root ").Quoted(identity.Command).String())
	}
	handler := lookup(identity.Command)
	if handler == nil {
		return refused(result, "missing-handler", 2,
			text.Reset().Str("root ").Quoted(identity.Command).
				Str(" has no registered handler").String())
	}

	result.Registered = true
	var output strings.Builder
	result.Code = leroot.Run(
		identity.Command,
		leroot.Answer(handler),
		slices.Clone(identity.Args),
		&output,
		&output,
	)
	result.Output = output.String()
	result.Completed = true
	return result
}

func refused(result verify.GateResult, kind string, code int, message string) verify.GateResult {
	result.Code = code
	result.Completed = false
	result.Failure = &verify.Failure{Kind: kind, Stage: result.Identity.Gate, Message: message}
	return result
}

func messageWithError(prefix string, err error) string {
	var text textbuf.Buffer
	return text.Str(prefix).Err(err).String()
}
