#!/bin/sh
# Unit tests for disk detection logic in the init script.
# Creates mock sysfs structures and tests find_target_disk.

set -e

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

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

# Extract find_target_disk but redirect /sys/block to our mock
find_target_disk_mock() {
    TARGET_DISK=""
    MOCK_SYS="$1"

    for dev in "$MOCK_SYS"/block/*; do
        [ -d "$dev" ] || continue
        name="$(basename "$dev")"

        case "$name" in
            loop*|ram*|dm-*|sr*|fd*|md*|zram*|mtdblock*)
                continue
                ;;
        esac

        if [ -f "$dev/removable" ]; then
            removable="$(cat "$dev/removable")"
            if [ "$removable" = "1" ]; then
                continue
            fi
        fi

        TARGET_DISK="/dev/$name"
        break
    done
}

# Test 1: single non-removable SATA disk
mkdir -p "$TMPDIR/test1/block/sda"
echo "0" > "$TMPDIR/test1/block/sda/removable"
find_target_disk_mock "$TMPDIR/test1"
assert_eq "sata-disk" "/dev/sda" "$TARGET_DISK"

# Test 2: single NVMe disk
mkdir -p "$TMPDIR/test2/block/nvme0n1"
echo "0" > "$TMPDIR/test2/block/nvme0n1/removable"
find_target_disk_mock "$TMPDIR/test2"
assert_eq "nvme-disk" "/dev/nvme0n1" "$TARGET_DISK"

# Test 3: single eMMC disk
mkdir -p "$TMPDIR/test3/block/mmcblk0"
echo "0" > "$TMPDIR/test3/block/mmcblk0/removable"
find_target_disk_mock "$TMPDIR/test3"
assert_eq "emmc-disk" "/dev/mmcblk0" "$TARGET_DISK"

# Test 4: removable USB disk skipped, fixed disk selected
mkdir -p "$TMPDIR/test4/block/sda" "$TMPDIR/test4/block/sdb"
echo "1" > "$TMPDIR/test4/block/sda/removable"
echo "0" > "$TMPDIR/test4/block/sdb/removable"
find_target_disk_mock "$TMPDIR/test4"
assert_eq "skip-removable" "/dev/sdb" "$TARGET_DISK"

# Test 5: all removable, no target found
mkdir -p "$TMPDIR/test5/block/sda"
echo "1" > "$TMPDIR/test5/block/sda/removable"
find_target_disk_mock "$TMPDIR/test5"
assert_eq "all-removable" "" "$TARGET_DISK"

# Test 6: no block devices at all
mkdir -p "$TMPDIR/test6/block"
find_target_disk_mock "$TMPDIR/test6"
assert_eq "no-devices" "" "$TARGET_DISK"

# Test 7: loop and ram devices skipped
mkdir -p "$TMPDIR/test7/block/loop0" "$TMPDIR/test7/block/ram0" "$TMPDIR/test7/block/sda"
echo "0" > "$TMPDIR/test7/block/loop0/removable"
echo "0" > "$TMPDIR/test7/block/ram0/removable"
echo "0" > "$TMPDIR/test7/block/sda/removable"
find_target_disk_mock "$TMPDIR/test7"
assert_eq "skip-virtual" "/dev/sda" "$TARGET_DISK"

# Test 8: zram and dm devices skipped
mkdir -p "$TMPDIR/test8/block/zram0" "$TMPDIR/test8/block/dm-0" "$TMPDIR/test8/block/sda"
echo "0" > "$TMPDIR/test8/block/zram0/removable"
echo "0" > "$TMPDIR/test8/block/dm-0/removable"
echo "0" > "$TMPDIR/test8/block/sda/removable"
find_target_disk_mock "$TMPDIR/test8"
assert_eq "skip-zram-dm" "/dev/sda" "$TARGET_DISK"

# Test 9: QEMU `virt` machine layout -- pflash mtdblock devices sort before the
# virtio disk in /sys/block and have no removable attribute, so without the
# mtdblock* skip the installer would target firmware flash instead of vda.
mkdir -p "$TMPDIR/test9/block/mtdblock0" "$TMPDIR/test9/block/mtdblock1" "$TMPDIR/test9/block/vda"
echo "0" > "$TMPDIR/test9/block/vda/removable"
find_target_disk_mock "$TMPDIR/test9"
assert_eq "skip-mtdblock" "/dev/vda" "$TARGET_DISK"

echo "---"
echo "disk-detect: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
