#!/bin/sh
# Build a Ze kernel inside Docker, QEMU, or any Linux build environment.
#
# Inputs (env):
#   LINUX_VERSION  - kernel version to build (default: 7.0.11)
#   ARCH           - target arch: arm64|amd64|x86_64 (default: arm64)
#   PROFILE        - config profile: qemu|hardware|hardware-kms|runtime (default: qemu)
#   JOBS           - parallel make jobs (default: nproc)
#   SRC_DIR        - directory containing config fragments (default: /src)
#   OUT_DIR        - directory for build output (default: /out)
#   MODULES        - yes|no, install modules and runtime artifacts (default: no)
#   PATCHES_DIR    - optional directory with a series file and patches
set -eu

LINUX_VERSION="${LINUX_VERSION:-7.0.11}"
ARCH="${ARCH:-arm64}"
PROFILE="${PROFILE:-qemu}"
JOBS="${JOBS:-$(nproc)}"
SRC_DIR="${SRC_DIR:-/src}"
OUT_DIR="${OUT_DIR:-/out}"
MODULES="${MODULES:-no}"
PATCHES_DIR="${PATCHES_DIR:-}"

case "$LINUX_VERSION" in
    ""|*[!0-9.]*|.*|*.) echo "unsupported LINUX_VERSION=$LINUX_VERSION (expected digits and dots)" >&2; exit 2 ;;
esac

case "$ARCH" in
    arm64) KERNEL_ARCH="arm64"; IMAGE_PATH="arch/arm64/boot/Image"; MAKE_TARGET="Image" ;;
    amd64|x86_64) KERNEL_ARCH="x86_64"; IMAGE_PATH="arch/x86/boot/bzImage"; MAKE_TARGET="bzImage" ;;
    *) echo "unsupported ARCH=$ARCH (expected arm64, amd64, or x86_64)" >&2; exit 2 ;;
esac

case "$MODULES" in
    yes|no) ;;
    *) echo "unsupported MODULES=$MODULES (expected yes or no)" >&2; exit 2 ;;
esac

case "$PROFILE" in
    qemu|hardware|runtime)
        profile_configs="${SRC_DIR}/${PROFILE}.config"
        ;;
    hardware-kms)
        profile_configs="${SRC_DIR}/hardware.config ${SRC_DIR}/hardware-kms.config"
        ;;
    *)
        echo "unsupported PROFILE=$PROFILE (expected qemu, hardware, hardware-kms, or runtime)" >&2
        exit 2
        ;;
esac

if [ ! -f "${SRC_DIR}/kernel.config" ]; then
    echo "missing config fragment: ${SRC_DIR}/kernel.config" >&2
    exit 2
fi
for f in $profile_configs; do
    if [ ! -f "$f" ]; then
        echo "missing config fragment: $f" >&2
        exit 2
    fi
done
if [ -n "$PATCHES_DIR" ] && [ ! -f "${PATCHES_DIR}/series" ]; then
    echo "missing patch series: ${PATCHES_DIR}/series" >&2
    exit 2
fi

series="v$(echo "$LINUX_VERSION" | cut -d. -f1).x"
tarball="linux-${LINUX_VERSION}.tar.xz"

cd /build
if [ -f "${tarball}" ]; then
    echo ">>> using pre-downloaded ${tarball}"
else
    echo ">>> downloading linux ${LINUX_VERSION}"
    wget -4 -q "https://cdn.kernel.org/pub/linux/kernel/${series}/${tarball}"
fi

BUILD_DIR="/tmp/kbuild"
if [ -d /proc ] && ! mountpoint -q "${BUILD_DIR}" 2>/dev/null; then
    mkdir -p "${BUILD_DIR}"
    if ! mount -t tmpfs -o size=5G tmpfs "${BUILD_DIR}" 2>/dev/null; then
        echo ">>> tmpfs unavailable (insufficient privileges), using regular directory"
    fi
fi

BUILD_TREE="${BUILD_DIR}/linux-${LINUX_VERSION}-${PROFILE}-${MODULES}"
CACHE_TAR="linux-${LINUX_VERSION}-${PROFILE}-${MODULES}.built.tar"
if [ -n "$PATCHES_DIR" ]; then
    rm -rf "${BUILD_TREE}"
elif [ -d "${BUILD_TREE}" ] && [ -f "${BUILD_TREE}/scripts/Kbuild.include" ]; then
    echo ">>> reusing existing source tree ${BUILD_TREE}"
elif [ -f "${CACHE_TAR}" ]; then
    echo ">>> restoring cached build tree from ${CACHE_TAR}"
    tar xf "${CACHE_TAR}" -C "${BUILD_DIR}"
fi
if [ ! -f "${BUILD_TREE}/scripts/Kbuild.include" ]; then
    rm -rf "${BUILD_TREE}" "${BUILD_DIR}/linux-${LINUX_VERSION}"
    echo ">>> extracting to ${BUILD_TREE}"
    tar xf "$tarball" -C "${BUILD_DIR}"
    mv "${BUILD_DIR}/linux-${LINUX_VERSION}" "${BUILD_TREE}"
fi
cd "${BUILD_TREE}"

if [ -n "$PATCHES_DIR" ]; then
    echo ">>> applying patches from ${PATCHES_DIR}"
    while IFS= read -r patch_name; do
        case "$patch_name" in
            ""|'#'*) continue ;;
            */*|*'..'*) echo "invalid patch name in series: $patch_name" >&2; exit 2 ;;
        esac
        patch_file="${PATCHES_DIR}/${patch_name}"
        if [ ! -f "$patch_file" ]; then
            echo "missing patch: $patch_file" >&2
            exit 2
        fi
        patch -p1 < "$patch_file"
    done < "${PATCHES_DIR}/series"
fi

echo ">>> configuring (defconfig + kernel.config + ${PROFILE} profile) for ${ARCH}"
make ARCH="$KERNEL_ARCH" defconfig
# shellcheck disable=SC2086 # intentional word-splitting on profile_configs
./scripts/kconfig/merge_config.sh -m .config "${SRC_DIR}/kernel.config" $profile_configs

FIRMWARE_DIR="${FIRMWARE_DIR:-}"
if [ "$PROFILE" = "hardware-kms" ] && [ -n "$FIRMWARE_DIR" ]; then
    FW_LIST="i915/adlp_dmc.bin"
    echo ">>> embedding firmware from ${FIRMWARE_DIR}"
    for blob in $FW_LIST; do
        if [ ! -f "${FIRMWARE_DIR}/${blob}" ]; then
            echo "FATAL: firmware ${blob} not found in ${FIRMWARE_DIR}" >&2
            exit 1
        fi
        echo "  ${blob}: $(wc -c < "${FIRMWARE_DIR}/${blob}") bytes"
    done
    cat >> .config <<FWEOF
CONFIG_EXTRA_FIRMWARE="${FW_LIST}"
CONFIG_EXTRA_FIRMWARE_DIR="${FIRMWARE_DIR}"
FWEOF
fi

make ARCH="$KERNEL_ARCH" olddefconfig

require_yes() {
    opt=$1
    context=$2
    if ! grep -q "^${opt}=y" .config; then
        echo "FATAL: ${opt} did not resolve to =y (${context})" >&2
        grep "${opt}" .config >&2 || true
        exit 1
    fi
}

echo ">>> verifying required built-in options"
for opt in CONFIG_IP_PNP_DHCP CONFIG_EXT4_FS CONFIG_BLK_DEV_INITRD CONFIG_DEVTMPFS_MOUNT; do
    require_yes "$opt" "required for all profiles"
done

if [ "$PROFILE" = "qemu" ]; then
    for opt in CONFIG_VIRTIO_NET CONFIG_VIRTIO_BLK; do
        require_yes "$opt" "required for qemu profile"
    done
fi

if [ "$PROFILE" = "hardware" ] || [ "$PROFILE" = "hardware-kms" ]; then
    for opt in CONFIG_EFI CONFIG_EFI_STUB CONFIG_FB_EFI CONFIG_FRAMEBUFFER_CONSOLE \
               CONFIG_E1000E CONFIG_IGB CONFIG_IGC CONFIG_R8169 \
               CONFIG_SATA_AHCI CONFIG_BLK_DEV_NVME \
               CONFIG_BLK_DEV_LOOP CONFIG_VFAT_FS CONFIG_EXFAT_FS; do
        require_yes "$opt" "required for ${PROFILE} profile"
    done
fi

if [ "$PROFILE" = "hardware-kms" ]; then
    for opt in CONFIG_DRM_KMS_HELPER CONFIG_DRM_I915 CONFIG_BACKLIGHT_CLASS_DEVICE; do
        require_yes "$opt" "required for hardware-kms profile"
    done
    if ! grep -q '^CONFIG_EXTRA_FIRMWARE=' .config; then
        echo "FATAL: CONFIG_EXTRA_FIRMWARE not set (i915 needs embedded firmware for display)" >&2
        exit 1
    fi
fi

if [ "$PROFILE" = "runtime" ]; then
    for opt in CONFIG_MODULES CONFIG_PPP CONFIG_PPPOL2TP CONFIG_L2TP CONFIG_L2TP_V3 \
               CONFIG_L2TP_NETLINK CONFIG_DEVTMPFS_MOUNT CONFIG_BLK_DEV_INITRD \
               CONFIG_VIRTIO_NET; do
        require_yes "$opt" "required for runtime profile"
    done
fi

echo ">>> building ${MAKE_TARGET} with -j${JOBS} (profile=${PROFILE}, modules=${MODULES})"
if [ "$MODULES" = "yes" ]; then
    make ARCH="$KERNEL_ARCH" -j"$JOBS" "$MAKE_TARGET" modules
else
    make ARCH="$KERNEL_ARCH" -j"$JOBS" "$MAKE_TARGET"
fi

mkdir -p "${OUT_DIR}"
cp .config "${OUT_DIR}/config"
if [ "$MODULES" = "yes" ]; then
    cp "$IMAGE_PATH" "${OUT_DIR}/vmlinuz"
    make ARCH="$KERNEL_ARCH" INSTALL_MOD_PATH="${OUT_DIR}" modules_install
    rm -f "${OUT_DIR}"/lib/modules/*/build "${OUT_DIR}"/lib/modules/*/source
    if [ "$KERNEL_ARCH" = "arm64" ] && [ -d arch/arm64/boot/dts ]; then
        find arch/arm64/boot/dts -type f -name '*.dtb' -exec cp -f {} "${OUT_DIR}/" \;
    fi
    if [ -d overlays ]; then
        rm -rf "${OUT_DIR}/overlays"
        cp -R overlays "${OUT_DIR}/"
    elif [ -d arch/arm64/boot/dts/overlays ]; then
        rm -rf "${OUT_DIR}/overlays"
        cp -R arch/arm64/boot/dts/overlays "${OUT_DIR}/"
    fi
    echo ">>> done: ${OUT_DIR}/vmlinuz (profile=${PROFILE})"
else
    cp "$IMAGE_PATH" "${OUT_DIR}/Image"
    if [ -z "$PATCHES_DIR" ]; then
        echo ">>> caching build tree to /build/${CACHE_TAR}"
        tar cf "/build/${CACHE_TAR}" -C "${BUILD_DIR}" "$(basename "${BUILD_TREE}")"
    fi
    echo ">>> done: ${OUT_DIR}/Image ($(du -h "${OUT_DIR}/Image" | cut -f1), profile=${PROFILE})"
fi
