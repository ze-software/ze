#!/bin/sh
# Unit tests for Ventoy/loop-mount ISO discovery in the init script.
# Mocks FAT partition scanning and loop-mount to test try_loop_mount_iso
# logic without real block devices or filesystems.
#
# Mock structure:
#   MOCK_PARTS/<partition>/   FAT partition contents (*.iso files)
#   MOCK_ISO/<iso-filename>/  loop-mounted ISO contents (ze-install/ tree)

set -e

PASS=0
FAIL=0

assert_eq() {
    label="$1"
    expected="$2"
    actual="$3"
    if [ "$expected" = "$actual" ]; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: $label: expected '$expected', got '$actual'"
        FAIL=$((FAIL + 1))
    fi
}

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

is_skipped_disk_name_mock() {
    case "$1" in
        loop*|ram*|dm-*|sr*|fd*|md*|zram*|mtdblock*) return 0 ;;
    esac
    return 1
}

disk_name_from_path_mock() {
    name="${1#/dev/}"
    case "$name" in
        nvme*p[0-9]*|mmcblk*p[0-9]*) echo "${name%p[0-9]*}" ;;
        sd*[0-9]|vd*[0-9]|xvd*[0-9]|hd*[0-9])
            while :; do
                case "$name" in
                    *[0-9]) name="${name%?}" ;;
                    *) break ;;
                esac
            done
            echo "$name"
            ;;
        *) echo "$name" ;;
    esac
}

# Mock try_loop_mount_iso: replaces mount/losetup/mknod with filesystem ops.
#
# Instead of mounting FAT partitions and loop-mounting ISOs, the mock reads
# from a directory tree under MOCK_PARTS (simulated FAT contents) and
# MOCK_ISO (simulated loop-mounted ISO contents). Each *.iso under
# MOCK_PARTS is a plain marker file; the mock resolves its content via
# MOCK_ISO/<basename>/.
try_loop_mount_iso_mock() {
    MOCK_SYS="$1"
    MOCK_PARTS="$2"
    MOCK_ISO="$3"
    ZE_IMAGE="$4"
    ZE_MEDIA_ID="$5"
    ISO_SOURCE_DEV=""
    ISO_SOURCE_DISK=""
    VENTOY_USB_DISK=""

    for dev in "$MOCK_SYS"/block/*; do
        [ -d "$dev" ] || continue
        name="$(basename "$dev")"
        is_skipped_disk_name_mock "$name" && continue

        for part_dir in "$MOCK_PARTS"/"$name"[0-9]* "$MOCK_PARTS"/"$name"p[0-9]*; do
            [ -d "$part_dir" ] || continue

            for iso_path in "$part_dir"/*.iso "$part_dir"/*/*.iso; do
                [ -f "$iso_path" ] || continue
                iso_name="$(basename "$iso_path")"

                iso_content="$MOCK_ISO/$iso_name"
                [ -d "$iso_content" ] || continue

                media_id_path="$iso_content/ze-install/media-id"
                if [ -f "$iso_content/ze-install/manifest.json" ] && \
                   [ -f "$iso_content/ze-install/images/$ZE_IMAGE" ] && \
                   [ -f "$media_id_path" ]; then
                    candidate_media_id="$(cat "$media_id_path" 2>/dev/null)"
                    if [ "$candidate_media_id" = "$ZE_MEDIA_ID" ]; then
                        ISO_SOURCE_DEV="/dev/loop0"
                        ISO_SOURCE_DISK="$(disk_name_from_path_mock "/dev/loop0")"
                        VENTOY_USB_DISK="$name"
                        return 0
                    fi
                fi
            done
        done
    done

    return 1
}

make_iso_content() {
    root="$1"
    iso_name="$2"
    image="$3"
    media_id="$4"
    mkdir -p "$root/$iso_name/ze-install/images"
    : > "$root/$iso_name/ze-install/manifest.json"
    printf '%s\n' "$media_id" > "$root/$iso_name/ze-install/media-id"
    : > "$root/$iso_name/ze-install/images/$image"
}

MEDIA_ID="0123456789abcdef0123456789abcdef"
WRONG_ID="fedcba9876543210fedcba9876543210"


# Test 1: ISO on FAT partition at top level.
mkdir -p "$TMPDIR/t1/sys/block/sda" "$TMPDIR/t1/parts/sda1"
: > "$TMPDIR/t1/parts/sda1/ze-install.iso"
make_iso_content "$TMPDIR/t1/iso" ze-install.iso ze.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t1/sys" "$TMPDIR/t1/parts" "$TMPDIR/t1/iso" ze.img "$MEDIA_ID"
assert_eq "fat-toplevel-ventoy-disk" "sda" "$VENTOY_USB_DISK"
assert_eq "fat-toplevel-source-dev" "/dev/loop0" "$ISO_SOURCE_DEV"
assert_eq "fat-toplevel-source-disk" "loop0" "$ISO_SOURCE_DISK"

# Test 2: ISO in subdirectory of FAT partition (one level deep).
mkdir -p "$TMPDIR/t2/sys/block/sdb" "$TMPDIR/t2/parts/sdb1/isos"
: > "$TMPDIR/t2/parts/sdb1/isos/ze-install.iso"
make_iso_content "$TMPDIR/t2/iso" ze-install.iso ze.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t2/sys" "$TMPDIR/t2/parts" "$TMPDIR/t2/iso" ze.img "$MEDIA_ID"
assert_eq "fat-subdir-ventoy-disk" "sdb" "$VENTOY_USB_DISK"
assert_eq "fat-subdir-source-dev" "/dev/loop0" "$ISO_SOURCE_DEV"

# Test 3: no ISO files on any partition.
mkdir -p "$TMPDIR/t3/sys/block/sda" "$TMPDIR/t3/parts/sda1" "$TMPDIR/t3/iso"
if try_loop_mount_iso_mock "$TMPDIR/t3/sys" "$TMPDIR/t3/parts" "$TMPDIR/t3/iso" ze.img "$MEDIA_ID"; then
    assert_eq "no-iso-files" "fail" "ok"
else
    assert_eq "no-iso-files" "fail" "fail"
fi

# Test 4: ISO exists but wrong media-id.
mkdir -p "$TMPDIR/t4/sys/block/sda" "$TMPDIR/t4/parts/sda1"
: > "$TMPDIR/t4/parts/sda1/ze-install.iso"
make_iso_content "$TMPDIR/t4/iso" ze-install.iso ze.img "$WRONG_ID"
if try_loop_mount_iso_mock "$TMPDIR/t4/sys" "$TMPDIR/t4/parts" "$TMPDIR/t4/iso" ze.img "$MEDIA_ID"; then
    assert_eq "wrong-media-id" "fail" "ok"
else
    assert_eq "wrong-media-id" "fail" "fail"
fi

# Test 5: ISO exists but missing manifest.json.
mkdir -p "$TMPDIR/t5/sys/block/sda" "$TMPDIR/t5/parts/sda1"
: > "$TMPDIR/t5/parts/sda1/ze-install.iso"
mkdir -p "$TMPDIR/t5/iso/ze-install.iso/ze-install/images"
printf '%s\n' "$MEDIA_ID" > "$TMPDIR/t5/iso/ze-install.iso/ze-install/media-id"
: > "$TMPDIR/t5/iso/ze-install.iso/ze-install/images/ze.img"
if try_loop_mount_iso_mock "$TMPDIR/t5/sys" "$TMPDIR/t5/parts" "$TMPDIR/t5/iso" ze.img "$MEDIA_ID"; then
    assert_eq "missing-manifest" "fail" "ok"
else
    assert_eq "missing-manifest" "fail" "fail"
fi

# Test 6: ISO exists but missing image file.
mkdir -p "$TMPDIR/t6/sys/block/sda" "$TMPDIR/t6/parts/sda1"
: > "$TMPDIR/t6/parts/sda1/ze-install.iso"
mkdir -p "$TMPDIR/t6/iso/ze-install.iso/ze-install/images"
: > "$TMPDIR/t6/iso/ze-install.iso/ze-install/manifest.json"
printf '%s\n' "$MEDIA_ID" > "$TMPDIR/t6/iso/ze-install.iso/ze-install/media-id"
if try_loop_mount_iso_mock "$TMPDIR/t6/sys" "$TMPDIR/t6/parts" "$TMPDIR/t6/iso" ze.img "$MEDIA_ID"; then
    assert_eq "missing-image" "fail" "ok"
else
    assert_eq "missing-image" "fail" "fail"
fi

# Test 7: first ISO has wrong media-id, second ISO matches.
mkdir -p "$TMPDIR/t7/sys/block/sda" "$TMPDIR/t7/parts/sda1"
: > "$TMPDIR/t7/parts/sda1/other.iso"
: > "$TMPDIR/t7/parts/sda1/ze-install.iso"
make_iso_content "$TMPDIR/t7/iso" other.iso ze.img "$WRONG_ID"
make_iso_content "$TMPDIR/t7/iso" ze-install.iso ze.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t7/sys" "$TMPDIR/t7/parts" "$TMPDIR/t7/iso" ze.img "$MEDIA_ID"
assert_eq "second-iso-matches" "sda" "$VENTOY_USB_DISK"

# Test 8: virtual devices (loop*, ram*) are skipped.
mkdir -p "$TMPDIR/t8/sys/block/loop0" "$TMPDIR/t8/sys/block/ram0"
mkdir -p "$TMPDIR/t8/parts/loop01" "$TMPDIR/t8/parts/ram01"
: > "$TMPDIR/t8/parts/loop01/ze-install.iso"
: > "$TMPDIR/t8/parts/ram01/ze-install.iso"
make_iso_content "$TMPDIR/t8/iso" ze-install.iso ze.img "$MEDIA_ID"
if try_loop_mount_iso_mock "$TMPDIR/t8/sys" "$TMPDIR/t8/parts" "$TMPDIR/t8/iso" ze.img "$MEDIA_ID"; then
    assert_eq "skip-virtual" "fail" "ok"
else
    assert_eq "skip-virtual" "fail" "fail"
fi

# Test 9: NVMe partition naming (nvme0n1p1 pattern).
mkdir -p "$TMPDIR/t9/sys/block/nvme0n1" "$TMPDIR/t9/parts/nvme0n1p1"
: > "$TMPDIR/t9/parts/nvme0n1p1/ze-install.iso"
make_iso_content "$TMPDIR/t9/iso" ze-install.iso ze.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t9/sys" "$TMPDIR/t9/parts" "$TMPDIR/t9/iso" ze.img "$MEDIA_ID"
assert_eq "nvme-partition" "nvme0n1" "$VENTOY_USB_DISK"

# Test 10: ISO on second disk when first has no FAT partitions.
mkdir -p "$TMPDIR/t10/sys/block/sda" "$TMPDIR/t10/sys/block/sdb"
mkdir -p "$TMPDIR/t10/parts/sdb1"
: > "$TMPDIR/t10/parts/sdb1/ze-install.iso"
make_iso_content "$TMPDIR/t10/iso" ze-install.iso ze.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t10/sys" "$TMPDIR/t10/parts" "$TMPDIR/t10/iso" ze.img "$MEDIA_ID"
assert_eq "second-disk" "sdb" "$VENTOY_USB_DISK"

# Test 11: ISO content directory missing (ISO not a valid ze installer).
mkdir -p "$TMPDIR/t11/sys/block/sda" "$TMPDIR/t11/parts/sda1" "$TMPDIR/t11/iso"
: > "$TMPDIR/t11/parts/sda1/random.iso"
if try_loop_mount_iso_mock "$TMPDIR/t11/sys" "$TMPDIR/t11/parts" "$TMPDIR/t11/iso" ze.img "$MEDIA_ID"; then
    assert_eq "non-ze-iso" "fail" "ok"
else
    assert_eq "non-ze-iso" "fail" "fail"
fi

# Test 12: custom image name.
mkdir -p "$TMPDIR/t12/sys/block/sda" "$TMPDIR/t12/parts/sda1"
: > "$TMPDIR/t12/parts/sda1/ze-install.iso"
make_iso_content "$TMPDIR/t12/iso" ze-install.iso ze-custom.img "$MEDIA_ID"
try_loop_mount_iso_mock "$TMPDIR/t12/sys" "$TMPDIR/t12/parts" "$TMPDIR/t12/iso" ze-custom.img "$MEDIA_ID"
assert_eq "custom-image-name" "sda" "$VENTOY_USB_DISK"


echo "---"
echo "ventoy-detect: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
