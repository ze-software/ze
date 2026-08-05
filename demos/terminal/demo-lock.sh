#!/usr/bin/env bash
# Hold the demo state tree while one demo runs.
#
# Every demo run owns tmp/terminal-demos/state/<demo-id>: run.sh deletes that
# directory and `ze init` refuses to write over a database that is already
# there. Two runs at once delete and re-initialise each other's database, and
# what they report points nowhere near the collision:
#
#   error: database already exists: .../cli-dashboard/config/database.zefs
#   error: read file/active/ze.conf: file does not exist
#   error: cannot connect to daemon: ... connect: connection refused
#   error: cannot connect to daemon: ... ssh: unable to authenticate
#
# The first of those is worse than the rest, because run.sh sends `ze init`
# output to a log file: the validator then exits non-zero with an empty
# terminal, and the render prints only the docker command that failed.
#
# The lock file sits on the mounted repository, so a container and the harness
# that starts it contend for one inode. render.py takes the same lock on the
# host and sets ZE_DEMO_LOCK_HELD in the container, which makes this file a
# passthrough there. Without that flag the two would deadlock.
#
# SOURCE this file and call demo_lock_acquire before the run touches anything
# shared. The demo shell's prompt is an exported PS1, and a non-interactive
# bash drops PS1 from the environment it passes on, so a wrapper SCRIPT between
# the entrypoint and vhs leaves vhs to paint its own `> ` prompt. Every tape
# then waits for a `$ ` that never comes. Running this file directly stays
# supported for tests, which do not care about the prompt.

demo_lock_acquire() {
    local lock_dir lock_file wait_seconds status

    if [[ -n "${ZE_DEMO_LOCK_HELD:-}" ]]; then
        return 0
    fi

    lock_dir=${ZE_DEMO_LOCK_DIR:-/src/tmp/terminal-demos}
    lock_file="${lock_dir}/demo-run.lock"
    wait_seconds=${ZE_DEMO_LOCK_WAIT:-1800}

    if ! mkdir -p "${lock_dir}"; then
        printf 'error: cannot create the demo lock directory %s\n' "${lock_dir}" >&2
        return 1
    fi

    exec 9>"${lock_file}"
    # The container runs as root and the host harness takes the same lock, so
    # the file must stay readable for the user that owns the repository.
    chmod 0644 "${lock_file}" 2>/dev/null || true
    status=0
    flock -w "${wait_seconds}" 9 || status=$?
    if [[ ${status} -eq 1 ]]; then
        printf 'error: another demo run held %s for %s seconds\n' \
            "${lock_file}" "${wait_seconds}" >&2
        return 1
    fi
    if [[ ${status} -ne 0 ]]; then
        printf 'error: flock %s failed with status %s\n' "${lock_file}" "${status}" >&2
        return "${status}"
    fi

    export ZE_DEMO_LOCK_HELD=1
    return 0
}

demo_lock_run() {
    local status

    if [[ $# -eq 0 ]]; then
        echo "usage: demo_lock_run <command> [argument ...]" >&2
        return 2
    fi

    demo_lock_acquire || return $?

    status=0
    "$@" || status=$?
    return "${status}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    demo_lock_run "$@"
    exit $?
fi
