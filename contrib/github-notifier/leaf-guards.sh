# Production leaf checks shared by install.sh and its isolated refusal proof.
# The caller must hold a root-owned non-writable installation namespace.

notifier_leaf_error() { printf '%s\n' "$1" >&2; return 1; }

notifier_installed_executable_safe() {
    if [ -L "$1" ]; then
        notifier_leaf_error 'The installed executable must not be a symbolic link. Inspect the release directory before installation.'
        return 1
    fi
    if [ ! -e "$1" ]; then return 0; fi
    if [ ! -f "$1" ]; then
        notifier_leaf_error 'The installed executable must be a regular file. Inspect the release directory before installation.'
        return 1
    fi
    if [ "$(stat -c %u "$1")" -ne 0 ]; then
        notifier_leaf_error 'An existing executable is not root-owned. Preserve it for investigation rather than reusing it.'
        return 1
    fi
    if [ "$(stat -c %a "$1")" != 755 ]; then
        notifier_leaf_error 'An existing executable has unexpected permissions. Inspect it rather than adopting possibly writable bytes.'
        return 1
    fi
}

notifier_build_record_safe() {
    if [ -L "$1" ]; then
        notifier_leaf_error 'The existing build record is a symbolic link. Inspect it before installation.'
        return 1
    fi
    if [ ! -e "$1" ]; then return 0; fi
    if [ ! -f "$1" ]; then
        notifier_leaf_error 'The existing build record is not a regular file. Inspect it before installation.'
        return 1
    fi
    if [ "$(stat -c %u "$1")" -ne 0 ]; then
        notifier_leaf_error 'The existing build record is not root-owned. Inspect it before installation.'
        return 1
    fi
    if [ "$(stat -c %a "$1")" != 644 ]; then
        notifier_leaf_error 'The existing build record has unexpected permissions. Inspect it before installation.'
        return 1
    fi
}
