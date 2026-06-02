#!/bin/sh
# Build the ze installer kernel inside the build container.
#
# Downloads pinned Linux source, merges kernel.config onto defconfig, verifies
# the must-be-builtin options actually resolved to =y, builds the kernel Image,
# and copies it (plus the resolved .config) to /out.
#
# Inputs (env): LINUX_VERSION, ARCH (arm64|amd64|x86_64), JOBS.
# Mounts: /src (this dir, ro) for kernel.config; /out for the artifacts.
set -eu

LINUX_VERSION="${LINUX_VERSION:-6.6.58}"
ARCH="${ARCH:-arm64}"
JOBS="${JOBS:-$(nproc)}"

case "$ARCH" in
    arm64)      KERNEL_ARCH="arm64";  IMAGE_PATH="arch/arm64/boot/Image";  MAKE_TARGET="Image" ;;
    amd64|x86_64) KERNEL_ARCH="x86_64"; IMAGE_PATH="arch/x86/boot/bzImage"; MAKE_TARGET="bzImage" ;;
    *) echo "unsupported ARCH=$ARCH (expected arm64, amd64, or x86_64)" >&2; exit 2 ;;
esac

series="v$(echo "$LINUX_VERSION" | cut -d. -f1).x"
tarball="linux-${LINUX_VERSION}.tar.xz"

cd /build
echo ">>> downloading linux ${LINUX_VERSION}"
wget -q "https://cdn.kernel.org/pub/linux/kernel/${series}/${tarball}"
tar xf "$tarball"
cd "linux-${LINUX_VERSION}"

echo ">>> configuring (defconfig + kernel.config) for ${ARCH}"
make ARCH="$KERNEL_ARCH" defconfig
./scripts/kconfig/merge_config.sh -m .config /src/kernel.config
make ARCH="$KERNEL_ARCH" olddefconfig

echo ">>> verifying required built-in options"
for opt in CONFIG_IP_PNP_DHCP CONFIG_VIRTIO_NET CONFIG_VIRTIO_BLK \
           CONFIG_EXT4_FS CONFIG_BLK_DEV_INITRD CONFIG_DEVTMPFS_MOUNT; do
    if ! grep -q "^${opt}=y" .config; then
        echo "FATAL: ${opt} did not resolve to =y" >&2
        grep "${opt}" .config >&2 || true
        exit 1
    fi
done

echo ">>> building ${MAKE_TARGET} with -j${JOBS}"
make ARCH="$KERNEL_ARCH" -j"$JOBS" "$MAKE_TARGET"

mkdir -p /out
cp "$IMAGE_PATH" /out/Image
cp .config /out/config
echo ">>> done: /out/Image ($(du -h /out/Image | cut -f1))"
