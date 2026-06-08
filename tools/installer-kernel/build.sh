#!/bin/sh
# Build the ze installer kernel inside the build container.
#
# Downloads pinned Linux source, merges kernel.config + profile config onto
# defconfig, verifies the must-be-builtin options actually resolved to =y,
# builds the kernel Image, and copies it (plus the resolved .config) to /out.
#
# Inputs (env): LINUX_VERSION, ARCH (arm64|amd64|x86_64), PROFILE (qemu|hardware|hardware-kms), JOBS.
# Mounts: /src (this dir, ro) for config fragments; /out for the artifacts.
set -eu

LINUX_VERSION="${LINUX_VERSION:-7.0.11}"
ARCH="${ARCH:-arm64}"
PROFILE="${PROFILE:-qemu}"
JOBS="${JOBS:-$(nproc)}"

case "$ARCH" in
    arm64)      KERNEL_ARCH="arm64";  IMAGE_PATH="arch/arm64/boot/Image";  MAKE_TARGET="Image" ;;
    amd64|x86_64) KERNEL_ARCH="x86_64"; IMAGE_PATH="arch/x86/boot/bzImage"; MAKE_TARGET="bzImage" ;;
    *) echo "unsupported ARCH=$ARCH (expected arm64, amd64, or x86_64)" >&2; exit 2 ;;
esac

case "$PROFILE" in
    qemu|hardware)
        profile_configs="/src/${PROFILE}.config"
        ;;
    hardware-kms)
        profile_configs="/src/hardware.config /src/hardware-kms.config"
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
echo ">>> downloading linux ${LINUX_VERSION}"
wget -q "https://cdn.kernel.org/pub/linux/kernel/${series}/${tarball}"
tar xf "$tarball"
cd "linux-${LINUX_VERSION}"

echo ">>> configuring (defconfig + kernel.config + ${PROFILE} profile) for ${ARCH}"
make ARCH="$KERNEL_ARCH" defconfig
# shellcheck disable=SC2086 # intentional word-splitting on profile_configs
./scripts/kconfig/merge_config.sh -m .config /src/kernel.config $profile_configs
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
               CONFIG_SATA_AHCI CONFIG_BLK_DEV_NVME; do
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

mkdir -p /out
cp "$IMAGE_PATH" /out/Image
cp .config /out/config
echo ">>> done: /out/Image ($(du -h /out/Image | cut -f1), profile=${PROFILE})"
