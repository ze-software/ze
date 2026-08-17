#!/usr/bin/env bash
set -euo pipefail

VHS_VERSION="${VHS_VERSION:-v0.11.0}"
TTYD_VERSION="${TTYD_VERSION:-1.7.7}"

fail() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

as_root() {
    if [[ "$(id -u)" == 0 ]]; then
        "$@"
    else
        command -v sudo >/dev/null 2>&1 || fail "sudo is required to install system packages"
        sudo "$@"
    fi
}

install_macos_dependencies() {
    command -v brew >/dev/null 2>&1 || fail "Homebrew is required on macOS"
    brew list ffmpeg >/dev/null 2>&1 || brew install ffmpeg
    if command -v ttyd >/dev/null 2>&1 && ttyd --version >/dev/null 2>&1; then
        return
    fi
    if brew list ttyd >/dev/null 2>&1; then
        brew reinstall ttyd
    else
        brew install ttyd
    fi
}

install_linux_ttyd() (
    local machine asset checksum temporary
    machine="$(uname -m)"
    case "$machine" in
        x86_64)
            asset="ttyd.x86_64"
            checksum="8a217c968aba172e0dbf3f34447218dc015bc4d5e59bf51db2f2cd12b7be4f55"
            ;;
        aarch64|arm64)
            asset="ttyd.aarch64"
            checksum="b38acadd89d1d396a0f5649aa52c539edbad07f4bc7348b27b4f4b7219dd4165"
            ;;
        *)
            fail "unsupported Linux architecture for ttyd: $machine"
            ;;
    esac

    temporary="$(mktemp)"
    trap 'rm -f "$temporary"' EXIT
    curl --fail --location --silent --show-error \
        "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/${asset}" \
        --output "$temporary"
    printf '%s  %s\n' "$checksum" "$temporary" | sha256sum --check --status
    as_root install -m 0755 "$temporary" /usr/local/bin/ttyd
)

install_ubuntu_dependencies() {
    [[ -r /etc/os-release ]] || fail "cannot identify this Linux distribution"
    # shellcheck disable=SC1091
    source /etc/os-release
    case "${ID:-}" in
        ubuntu|debian) ;;
        *) fail "native installation supports Ubuntu and Debian, found ${ID:-unknown}" ;;
    esac

    as_root apt-get update
    as_root apt-get install --yes ca-certificates curl ffmpeg
    if ! command -v ttyd >/dev/null 2>&1 || ! ttyd --version >/dev/null 2>&1; then
        install_linux_ttyd
    fi
}

case "$(uname -s)" in
    Darwin)
        install_macos_dependencies
        default_bin_dir="$(brew --prefix)/bin"
        ;;
    Linux)
        install_ubuntu_dependencies
        default_bin_dir="${HOME}/.local/bin"
        ;;
    *)
        fail "native installation supports macOS and Ubuntu only"
        ;;
esac

command -v go >/dev/null 2>&1 || fail "Go is required to install VHS"
install_dir="${VHS_BIN_DIR:-$default_bin_dir}"
mkdir -p "$install_dir"
CGO_ENABLED=0 GOBIN="$install_dir" go install "github.com/charmbracelet/vhs@${VHS_VERSION}"

"$install_dir/vhs" --version
ffmpeg -version | sed -n '1p'
ttyd --version
printf 'VHS and its native dependencies are installed in %s\n' "$install_dir"
