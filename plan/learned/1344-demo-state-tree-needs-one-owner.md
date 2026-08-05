# 1344 -- The Demo State Tree Needs One Owner, And A Failure Must Name Itself

## Context

On 2026-08-04 `make ze-terminal-demos` failed at `validating cli-dashboard...`
with a bare docker exit status and no validator output. The demo passes 30 times
out of 30 when it runs alone. The tree was not at fault, and neither were that
day's reactor and CLI commits.

Every demo run OWNS `tmp/terminal-demos/state/<demo-id>`. `run.sh` deletes that
directory and runs `ze init` over it. `runInit`
(`internal/plugins/init/main.go`) then refuses a database that is already there.
Nothing made that claim exclusive, so two runs at once deleted and
re-initialised each other's database. The check is right and the input is wrong:
a state directory another run owns.

Reproduced 6 times out of 20 on the shared path, 0 times out of 20 with a
private bind mount over `/src/tmp/terminal-demos/state`. A diagnostic container
watched `database.zefs` come back 1.1s to 2.9s AFTER its own `rm -rf`, with no
`ze` process of its own.

The collision reaches the operator wearing four faces. None of them names it:

- `database already exists`
- `read file/active/ze.conf: file does not exist`
- `connection refused` on 127.0.0.1:2222
- `ssh: unable to authenticate`, against a daemon that loaded the database its
  rival replaced

## Decisions

- One lock for every demo container (`demos/terminal/demo-lock.sh`), taken in
  `container-entrypoint.sh`. The entrypoint is the single process boundary of
  every demo container, so one edit covers rendering and validation, and no
  per-demo `run.sh` changes.
- The entrypoint takes it FIRST, before its own setup. `HOME` is on the mount
  and every container shares it, so `cp shell.sh "${HOME}/.bashrc"` truncates
  the prompt and the PATH of a shell another demo is already driving.
- The entrypoint SOURCES the lock and calls `demo_lock_acquire`. A wrapper
  script is another bash, and that costs the demo its prompt: see the fourth
  Gotcha.
- `render.py` takes the SAME lock on the host (`demo_lock`), because the
  harness owns files the container never sees: it writes
  `tmp/terminal-demos/render-tapes/<id>.tape`, removes it after the container
  exits, and rewrites the capture with ffmpeg. A container lock alone made that
  window minutes long rather than closing it.
- The container is told the lock is already held (`ZE_DEMO_LOCK_HELD`). Two
  acquisitions from two processes over one file wait for each other, so without
  the flag every render would deadlock. The same flag makes a nested call from
  inside a demo a passthrough.
- The harness lock covers the artifact manifest, not only the renders inside
  it. `render_selected` holds it across `load_artifact_manifest`, the renders
  and `write_artifact_manifest`, because two runs that each READ the manifest
  before either writes both publish their own view, and the second write drops
  the first run's demos. `demo_lock` is re-entrant in the process for the same
  reason: `run_demo` takes it again under that hold.
- The lock file lives on the mounted repository, not in the image. The
  container, the harness that starts it and a second `make ze-terminal-demos`
  then contend for one inode.
- `ze init --force` was refused. It converts a loud collision into a silent
  one. The two runs still share one database, one daemon and one SSH port for
  the rest of the demo, so the recording shows the wrong daemon's state.
- An ERR trap in `validate-common.sh` prints the logs the run captured. It hangs
  on ERR because a validator that starts a demo installs its own EXIT trap,
  which would replace an EXIT one. `errtrace` carries it into functions.
- `demos/terminal` joins `pythonTestRoots`
  (`scripts/dev/python_tests_test.go`). `test_render.py` had never run.

## Consequences

- Two demo containers on the same demo now pass 10 times out of 10. With the
  pre-fix entrypoint the same pairs lose one run of every pair, 3 times out of
  3. That is the discrimination proof.
- A failing validator now prints `init.log`, `import.log` and `daemon.log`. The
  first pre-fix failure it caught read `atomic rename ... database.zefs.init-tmp
  -> database.zefs: no such file or directory`, which was invisible before.
- Editing `demo-lock.sh`, `container-entrypoint.sh` or `validate-common.sh`
  changes `source_digest` for EVERY demo. All three are in `render.py`'s
  `SHARED_SOURCE_PATHS`, so any change forces a full re-render before `--check`
  passes again.
- Demo runs now serialise repo-wide. A hung demo makes the next one wait rather
  than corrupt it, up to `ZE_DEMO_LOCK_WAIT` (1800 seconds in the container).
  The harness waits 7200, because it holds the lock for a whole `--all` render
  rather than for one demo.
- Two `render.py --demo cli-dashboard --release` processes started together
  both exit 0. Before the host lock the second removed the tape the first was
  about to read.

## Gotchas

- **A silent non-zero exit is the expensive part, not the fault itself.**
  `run.sh` sends `ze init` and `ze config import` output to log files. Both are
  therefore invisible when they fail, and `render.py` prints only the docker
  command. Diagnosis needed a container that preserved the state directory of a
  failing run. The message was waiting in `init.log` the whole time.
- **A demo that passes alone proves nothing about a demo suite.** Isolation is
  what the repeat runs measured. Only pairs run at the same time reproduce this.
- **A non-interactive bash drops PS1 from the environment it passes on.** The
  demo shell's `$ ` prompt is the exported `PS1` in `container-entrypoint.sh`.
  vhs starts the pty shell with `bash --noprofile --norc`, so the environment is
  the ONLY source of that prompt. The first version of the lock
  was a wrapper SCRIPT, one bash between the entrypoint and vhs. vhs then
  painted its own purple `> ` prompt, and every tape ending in
  `Wait+Screen /\$ /` timed out on it. Validation passed 18 times out of 18
  while every render failed, because a validator never looks at the prompt.
  `demo_lock_run` is a sourced function for that reason.
- **Container processes ARE visible in the host process list, and their argv is
  the in-container path.** A `ze init` from the mounted repository shows as
  `./tmp/terminal-demos/bin/ze init`, while the same binary reached through PATH
  inside a container shows as `ze init`. Filtering the sample on the long path
  hides every container process and points the search at the wrong host.
- **The container runs as root, so the harness cannot open the lock file for
  writing.** The container creates `demo-run.lock` on the mount as root, and
  `render.py` runs as the user who owns the repository. `flock(2)` takes any
  open descriptor, so the harness opens it `O_RDONLY | O_CREAT` and the shell
  chmods it 0644. A `open("w")` there fails with EACCES on the second run of
  the day, and only then.
- **A straggler from an exited container is not the explanation.** When PID 1 of
  a container exits the kernel kills the rest of its PID namespace at once. A
  background writer measured across that boundary stops on the same tick.

## Files

- `demos/terminal/demo-lock.sh`: `demo_lock_acquire`, `demo_lock_run`,
  `ZE_DEMO_LOCK_DIR`, `ZE_DEMO_LOCK_WAIT` and `ZE_DEMO_LOCK_HELD`
- `demos/terminal/container-entrypoint.sh`: the lock precedes the setup
- `demos/terminal/render.py`: `demo_lock`, `LOCK_PATH`, and the
  `ZE_DEMO_LOCK_HELD` env the container is started with
- `demos/terminal/validate-common.sh`: `demo_report_logs` and its ERR trap
- `demos/terminal/test_render.py`: `DemoLockTest` (serialisation, the
  entrypoint wiring, and the environment the lock hands on),
  `ValidatorDiagnosticTest`
- `scripts/dev/python_tests_test.go`: `demos/terminal` as a Python test root
