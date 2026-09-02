// Design: docs/architecture/core-design.md -- registered in-process verification dispatch
//
// Package verifydispatch connects verifyworktree to le's local-data registry.
// It owns no command table: verifyengine.Identity names the tool and exact arguments.
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
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

var rootOverrideMu sync.Mutex

// RunAction runs one registered le action inside the requested checkout root.
func RunAction(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
	return dispatch(ctx, root, identity, leroot.Owns, leroot.LookupCommand, lepath.Root)
}

func dispatch(
	ctx context.Context,
	root string,
	identity verifyengine.Identity,
	owns func(string) bool,
	lookup func(string) registry.LocalDataHandler,
	resolveRoot func() (string, error),
) (result verifyengine.ActionResult) {
	identity.Args = slices.Clone(identity.Args)
	var text textbuf.Buffer
	result.Identity = identity
	if err := ctx.Err(); err != nil {
		return refused(result, "interrupted", verifyengine.Interrupted, err.Error())
	}
	if strings.TrimSpace(root) == "" {
		return refused(result, "root-missing", 2, "action runner received no checkout root")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("resolve action root: ", err))
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("open action root: ", err))
	}
	if !info.IsDir() {
		return refused(result, "root-missing", 2, "action root is not a directory")
	}

	rootOverrideMu.Lock()
	defer rootOverrideMu.Unlock()
	if err := ctx.Err(); err != nil {
		return refused(result, "interrupted", verifyengine.Interrupted, err.Error())
	}

	previousRoot := env.Get(lepath.RootKey)
	if err := env.Set(lepath.RootKey, absoluteRoot); err != nil {
		return refused(result, "root-override", 2, messageWithError("set action root: ", err))
	}
	defer func() {
		if err := env.Set(lepath.RootKey, previousRoot); err != nil {
			result = refused(result, "root-restore", 2, messageWithError("restore action root: ", err))
		}
	}()

	resolvedRoot, err := resolveRoot()
	if err != nil {
		return refused(result, "root-mismatch", 2, messageWithError("resolve overridden action root: ", err))
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return refused(result, "root-mismatch", 2, messageWithError("normalize overridden action root: ", err))
	}
	if filepath.Clean(resolvedRoot) != filepath.Clean(absoluteRoot) {
		message := text.Reset().Str("action root resolved to ").Quoted(resolvedRoot).
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
	var output textbuf.Buffer
	output.Reset()
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

func refused(result verifyengine.ActionResult, kind string, code int, message string) verifyengine.ActionResult {
	result.Code = code
	result.Completed = false
	result.Failure = &verifyengine.Failure{Kind: kind, Stage: result.Identity.Name, Message: message}
	return result
}

func messageWithError(prefix string, err error) string {
	var text textbuf.Buffer
	return text.Str(prefix).Err(err).String()
}
