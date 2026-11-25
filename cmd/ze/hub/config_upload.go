// Design: docs/architecture/appliance/remote-operations.md -- daemon-owned configuration mutations.
// Related: config_source.go -- the source-aware candidate/commit transaction.
// Related: main_reload.go -- reloadGate joins mutations before store release, and stands them down at daemon stop.

package hub

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configupload"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const uploadValidateFixBase = "ze config validate <file>"

// configMutationOwner owns one daemon generation's commit and reload lifetime.
// Its zero value is a starting daemon: every operation refuses until bind.
// bind publishes once; close seals admission and joins every admitted operation.
// It must not be copied after use. A nil owner is a wiring defect, not a mode.
type configMutationOwner struct {
	gate    reloadGate
	runtime atomic.Pointer[configMutationRuntime]
}

// configMutationRuntime is immutable after binding. Its window stays bound even
// after shutdown withdraws the public window. A nil window means this runtime
// has no confirmed-commit worker. A storeless daemon can reload its source,
// but refuses persistent writes. A nonnil store requires a source path.
type configMutationRuntime struct {
	store  storage.Storage
	path   string
	window *confirm.Window
	reload func(context.Context) error
}

// bind publishes the runtime once with its window, or nil when it has none.
// The caller MUST keep reload admitted until the bound window's worker stops.
func (o *configMutationOwner) bind(store storage.Storage, path string, window *confirm.Window, reload func(context.Context) error) error {
	if reload == nil {
		return errors.New("configuration mutation owner requires a daemon reload operation")
	}
	if store != nil {
		if path == "" {
			return errors.New("configuration mutation owner requires its persistent source path")
		}
	}
	if !o.runtime.CompareAndSwap(nil, &configMutationRuntime{store: store, path: path, window: window, reload: reload}) {
		return errors.New("configuration mutation owner runtime is already bound")
	}
	return nil
}

var errConfigRuntimeStarting = opererr.New("configuration runtime is still starting", "The daemon cannot apply a configuration before its reload operation is ready", "ze doctor")

// Reload runs the daemon's commit reload. Like Commit and Upload it runs under
// the gate's shutdown scope (reloadGate.admit), so the daemon's stop stands it
// down, and its caller is then answered that the stop interrupted the change
// (stoodDownAnswer).
func (o *configMutationOwner) Reload(ctx context.Context) error {
	ctx, leave, admitted := o.gate.admit(ctx)
	if !admitted {
		return errDaemonStopping
	}
	defer leave()
	runtime := o.runtime.Load()
	if runtime == nil {
		return errConfigRuntimeStarting
	}
	return stoodDownAnswer(ctx, runtime.reload(ctx))
}

// Commit is the source-aware commit operation shared with the web source editor
// and HTTP upload adapter. Their existing validation and HTTP behavior remain
// at that boundary. Admission precedes every store read and candidate write.
func (o *configMutationOwner) Commit(ctx context.Context, expected, content []byte) error {
	ctx, leave, admitted := o.gate.admit(ctx)
	if !admitted {
		return errDaemonStopping
	}
	defer leave()
	runtime := o.runtime.Load()
	if runtime == nil {
		return errConfigRuntimeStarting
	}
	return stoodDownAnswer(ctx, runtime.commit(ctx, expected, content))
}

// Upload validates a fully received replacement before using the same canonical
// commit as HTTP. Once admitted, it runs synchronously through rollback or source
// publication; disconnecting never leaves a detached configuration writer. A
// daemon stop is the one event that ends it early (reloadGate.standDown).
func (o *configMutationOwner) Upload(ctx context.Context, content []byte) error {
	ctx, leave, admitted := o.gate.admit(ctx)
	if !admitted {
		return errDaemonStopping
	}
	defer leave()
	runtime := o.runtime.Load()
	if runtime == nil {
		return errConfigRuntimeStarting
	}
	if len(content) == 0 {
		return opererr.New("configuration upload is empty", "No replacement was applied", uploadValidateFixBase)
	}
	if len(content) > configupload.MaxBytes {
		return opererr.New("configuration upload exceeds four MiB", "No replacement was applied", uploadValidateFixBase)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := zeconfigcmd.ValidateContent(string(content), runtime.path); err != nil {
		// Parser errors can contain the input line, including secrets. Keep them
		// on the operator's local validation path, never in the SSH receipt.
		return opererr.New("configuration upload failed validation", "The configuration was refused before publication; validate the source locally to inspect its errors", uploadValidationFix())
	}
	if runtime.store == nil {
		return opererr.New("configuration upload requires persistent storage", "This daemon cannot retain a replacement configuration", "ze start --help")
	}
	expected, err := storage.ReadConfigSource(runtime.store, runtime.path)
	if err != nil {
		return opererr.New("configuration upload could not read the accepted source", "No replacement was staged; check the device storage", "ze doctor")
	}
	if err := runtime.commit(ctx, expected, content); err != nil {
		// A managed device refuses the change at staging. That refusal names
		// the one fix that works, so it is answered as the freeze wrote it.
		if refusal, frozen := storage.FreezeRefusal(err); frozen {
			return refusal
		}
		// A daemon stop ended the commit. The answer carries no cause, as the
		// one below: the receipt never quotes the transaction's error.
		if stoodDown(ctx, err) {
			return opererr.New(stoodDownProblem, stoodDownWhy, stoodDownFix)
		}
		return opererr.New("configuration upload was not accepted", "The daemon could not complete its configuration transaction; inspect the live configuration and daemon diagnostics before retrying", "show config running")
	}
	return nil
}

func (r *configMutationRuntime) commit(ctx context.Context, expected, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return commitRuntimeConfig(r.store, r.path, r.path, expected, content, func() error {
		return r.reload(ctx)
	}, r.window)
}

func (o *configMutationOwner) close() { o.gate.close() }

// uploadValidationFix builds the local validation command for the SSH upload
// validation receipt. It carries the daemon's recorded --plugin arguments
// before `config validate` so local validation judges with the same plugin
// selection the upload met, and stays `ze config validate <file>` when none
// was recorded.
func uploadValidationFix() string {
	plugins := zeconfig.CommandLinePlugins()
	if len(plugins) == 0 {
		return uploadValidateFixBase
	}
	var fix textbuf.Buffer
	fix.Str("ze")
	for _, pluginName := range plugins {
		fix.Str(" --plugin ")
		command.WriteInvocationValue(&fix, pluginName)
	}
	fix.Str(" config validate <file>")
	return fix.String()
}
