#!/bin/sh
# Build the ze installer kernel inside a QEMU VM or any Linux environment.
#
# Downloads pinned Linux source, merges kernel.config + profile config onto
# defconfig, verifies the must-be-builtin options actually resolved to =y,
# builds the kernel Image, and copies it (plus the resolved .config) to OUT_DIR.
#
# Inputs (env):
#   LINUX_VERSION  - kernel version to build (default: 7.0.11)
#   ARCH           - target arch: arm64|amd64|x86_64 (default: arm64)
#   PROFILE        - config profile: qemu|hardware|hardware-kms (default: qemu)
#   JOBS           - parallel make jobs (default: nproc)
#   SRC_DIR        - directory containing config fragments (default: /src)
#   OUT_DIR        - directory for build output (default: /out)
set -eu

LINUX_VERSION="${LINUX_VERSION:-7.0.11}"
ARCH="${ARCH:-arm64}"
PROFILE="${PROFILE:-qemu}"
JOBS="${JOBS:-$(nproc)}"
SRC_DIR="${SRC_DIR:-/src}"
OUT_DIR="${OUT_DIR:-/out}"

case "$ARCH" in
    arm64)      KERNEL_ARCH="arm64";  IMAGE_PATH="arch/arm64/boot/Image";  MAKE_TARGET="Image" ;;
    amd64|x86_64) KERNEL_ARCH="x86_64"; IMAGE_PATH="arch/x86/boot/bzImage"; MAKE_TARGET="bzImage" ;;
    *) echo "unsupported ARCH=$ARCH (expected arm64, amd64, or x86_64)" >&2; exit 2 ;;
esac

case "$PROFILE" in
    qemu|hardware)
        profile_configs="${SRC_DIR}/${PROFILE}.config"
        ;;
    hardware-kms)
        profile_configs="${SRC_DIR}/hardware.config ${SRC_DIR}/hardware-kms.config"
        ;;
    *)
        echo "unsupported PROFILE=$PROFILE (expected qemu, hardware, or hardware-kms)" >&2
        exit 2
        ;;
esac

for f in $profile_configs; do
    if [ ! -f "$f" ]; then
        echo "missing config fragment: $f" >&2
        exit 2
    fi
done

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
    mount -t tmpfs -o size=5G tmpfs "${BUILD_DIR}"
fi

BUILD_TREE="${BUILD_DIR}/linux-${LINUX_VERSION}"
CACHE_TAR="linux-${LINUX_VERSION}.built.tar"
if [ -d "${BUILD_TREE}" ] && [ -f "${BUILD_TREE}/scripts/Kbuild.include" ]; then
    echo ">>> reusing existing source tree ${BUILD_TREE}"
elif [ -f "${CACHE_TAR}" ]; then
    echo ">>> restoring cached build tree from ${CACHE_TAR}"
    tar xf "${CACHE_TAR}" -C "${BUILD_DIR}"
else
    rm -rf "${BUILD_TREE}"
    echo ">>> extracting to ${BUILD_TREE}"
    tar xf "$tarball" -C "${BUILD_DIR}"
fi
cd "${BUILD_TREE}"

echo ">>> configuring (defconfig + kernel.config + ${PROFILE} profile) for ${ARCH}"
make ARCH="$KERNEL_ARCH" defconfig
# shellcheck disable=SC2086 # intentional word-splitting on profile_configs
./scripts/kconfig/merge_config.sh -m .config "${SRC_DIR}/kernel.config" $profile_configs
make ARCH="$KERNEL_ARCH" olddefconfig

echo ">>> verifying required built-in options"
for opt in CONFIG_IP_PNP_DHCP CONFIG_EXT4_FS CONFIG_BLK_DEV_INITRD CONFIG_DEVTMPFS_MOUNT; do
    if ! grep -q "^${opt}=y" .config; then
        echo "FATAL: ${opt} did not resolve to =y" >&2
        grep "${opt}" .config >&2 || true
        exit 1
    fi
done

if [ "$PROFILE" = "qemu" ]; then
    for opt in CONFIG_VIRTIO_NET CONFIG_VIRTIO_BLK; do
        if ! grep -q "^${opt}=y" .config; then
            echo "FATAL: ${opt} did not resolve to =y (required for qemu profile)" >&2
            grep "${opt}" .config >&2 || true
            exit 1
        fi
    done
fi

if [ "$PROFILE" = "hardware" ] || [ "$PROFILE" = "hardware-kms" ]; then
    for opt in CONFIG_EFI CONFIG_EFI_STUB CONFIG_FB_EFI CONFIG_FRAMEBUFFER_CONSOLE \
               CONFIG_E1000E CONFIG_IGB CONFIG_IGC CONFIG_R8169 \
               CONFIG_SATA_AHCI CONFIG_BLK_DEV_NVME \
               CONFIG_BLK_DEV_LOOP CONFIG_VFAT_FS CONFIG_EXFAT_FS; do
        if ! grep -q "^${opt}=y" .config; then
            echo "FATAL: ${opt} did not resolve to =y (required for ${PROFILE} profile)" >&2
            grep "${opt}" .config >&2 || true
            exit 1
        fi
    done
fi

if [ "$PROFILE" = "hardware-kms" ]; then
    for opt in CONFIG_DRM_KMS_HELPER CONFIG_DRM_I915 CONFIG_BACKLIGHT_CLASS_DEVICE; do
        if ! grep -q "^${opt}=y" .config; then
            echo "FATAL: ${opt} did not resolve to =y (required for hardware-kms profile)" >&2
            grep "${opt}" .config >&2 || true
            exit 1
        fi
    done
fi

echo ">>> building ${MAKE_TARGET} with -j${JOBS} (profile=${PROFILE})"
make ARCH="$KERNEL_ARCH" -j"$JOBS" "$MAKE_TARGET"

mkdir -p "${OUT_DIR}"
cp "$IMAGE_PATH" "${OUT_DIR}/Image"
cp .config "${OUT_DIR}/config"

echo ">>> caching build tree to /build/${CACHE_TAR}"
tar cf "/build/${CACHE_TAR}" -C /tmp "linux-${LINUX_VERSION}"

echo ">>> done: ${OUT_DIR}/Image ($(du -h "${OUT_DIR}/Image" | cut -f1), profile=${PROFILE})"
