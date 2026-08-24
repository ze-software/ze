#!/usr/bin/env bash
set -u

# The demo lock comes FIRST, before this script writes anything. HOME is on the
# mounted repository and every demo container shares it, so the setup below
# truncates and rewrites files that a running demo reads: `.bashrc` carries the
# prompt and the PATH of the shell the recorder drives. A lock taken after the
# setup would leave that window open (demos/terminal/demo-lock.sh).
#
# The lock is SOURCED, so the demo runs in this shell. A wrapper script would
# be a non-interactive bash, and that drops the exported PS1 below: the shell
# then paints its own prompt and every `Wait+Screen /\$ /` in a tape times out.

# The container runs as root, because six demos build network-namespace labs
# and `--user` drops every capability the manifest asks for: `docker run
# --user 1000:1000 --privileged` reports CapEff 0000000000000000, so `ip netns
# add` fails with EPERM. Root is not negotiable, so what root wrote on the
# shared mount has its ownership handed back here instead.
#
# The trap is installed BEFORE the lock, because demo-lock.sh opens
# /src/tmp/terminal-demos/demo-run.lock for writing and creates it as root. It
# covers an interrupted render as well as a clean one: a Ctrl-C that left the
# tree root-owned is what made a plain `cp` of tmp/ fail on the host.
#
# Only the two mounts the container writes are walked. /src/tmp/terminal-demos
# holds HOME, XDG_RUNTIME_DIR, the demo state trees, the browser video captures
# and the lock; the artifacts tree holds the recordings. Everything else under
# /src belongs to the host.
give_ownership_back() {
    [[ -n "${HOST_UID:-}" && -n "${HOST_GID:-}" ]] || return 0
    chown -R "${HOST_UID}:${HOST_GID}" /src/demos/terminal/artifacts 2>/dev/null || true
    chown -R "${HOST_UID}:${HOST_GID}" /src/tmp/terminal-demos 2>/dev/null || true
}
trap give_ownership_back EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

source /src/demos/terminal/demo-lock.sh
demo_lock_acquire || exit $?

export HOME=/src/tmp/terminal-demos/home
export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_RUNTIME_DIR=/src/tmp/terminal-demos/runtime
export PATH="/src/tmp/terminal-demos/bin:${PATH}"
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export TZ=UTC
export TERM=xterm-256color
export PS1='$ '
mkdir -p "${HOME}" "${XDG_CONFIG_HOME}" "${XDG_DATA_HOME}" "${XDG_RUNTIME_DIR}"
cp /src/demos/terminal/shell.sh "${HOME}/.bashrc"
mkdir -p "${HOME}/.ssh"
printf '%s\n' \
    'Host ze-demo' \
    '  HostName 127.0.0.1' \
    '  Port 2222' \
    '  User admin' \
    '  StrictHostKeyChecking no' \
    '  UserKnownHostsFile /dev/null' \
    >"${HOME}/.ssh/config"
chmod 600 "${HOME}/.ssh/config"
mkdir -p /root/.ssh
cp "${HOME}/.ssh/config" /root/.ssh/config

case "${1:-}" in
    *.tape)
        # In this shell, at the position the recorder has always been invoked
        # from, so it inherits the environment exported above. A wrapper script
        # here would be a non-interactive bash and would lose PS1.
        python3 /src/demos/terminal/pty-session.py --tape "$@"
        ;;
    *)
        "$@"
        ;;
esac
status=$?

exit "${status}"
