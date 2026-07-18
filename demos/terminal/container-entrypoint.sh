#!/usr/bin/env bash
set -u

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
        vhs "$@"
        ;;
    *)
        "$@"
        ;;
esac
status=$?

if [[ -n "${HOST_UID:-}" && -n "${HOST_GID:-}" ]]; then
    chown -R "${HOST_UID}:${HOST_GID}" /src/demos/terminal/artifacts 2>/dev/null || true
fi

exit "${status}"
