#!/usr/bin/env bash

export PATH="/src/tmp/terminal-demos/bin:${PATH}"
export HOME="/src/tmp/terminal-demos/home"
export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_RUNTIME_DIR="/src/tmp/terminal-demos/runtime"
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export TZ=UTC
export TERM=xterm-256color
export PS1='$ '

mkdir -p "${HOME}" "${XDG_CONFIG_HOME}" "${XDG_DATA_HOME}" "${XDG_RUNTIME_DIR}"
chmod 700 "${XDG_RUNTIME_DIR}"
cd /src
