#!/bin/sh
# Unit tests for disk detection logic in the init script.
# Creates mock sysfs structures and tests find_target_disk.

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
assert_valid_target_path() {
    label="$1"
    path="$2"
    if validate_target_path_mock "$path"; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: $label: expected '$path' to be accepted"
        FAIL=$((FAIL + 1))
    fi
}

assert_invalid_target_path() {
    label="$1"
    path="$2"
    if validate_target_path_mock "$path"; then
        echo "FAIL: $label: expected '$path' to be rejected"
        FAIL=$((FAIL + 1))
    else
        PASS=$((PASS + 1))
    fi
}


TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

VENTOY_USB_DISK=""

validate_decimal_mock() {
    case "$1" in
        ""|*[!0-9]*) return 1 ;;
    esac
    return 0
}

validate_target_path_mock() {
    case "$1" in
        /dev/*) ;;
        *) return 1 ;;
    esac
    name="${1#/dev/}"
    case "$name" in
        ""|*[!a-zA-Z0-9_-]*) return 1 ;;
        sd*[!a-z]*|vd*[!a-z]*|xvd*[!a-z]*|hd*[!a-z]*) return 1 ;;
        sd?*|vd?*|xvd?*|hd?*) return 0 ;;
        nvme*n*)
            controller="${name#nvme}"
            namespace="${controller#*n}"
            controller="${controller%%n*}"
            validate_decimal_mock "$controller" && validate_decimal_mock "$namespace"
            return
            ;;
        mmcblk*)
            index="${name#mmcblk}"
            validate_decimal_mock "$index"
            return
            ;;
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

is_skipped_disk_name_mock() {
    case "$1" in
        loop*|ram*|dm-*|sr*|fd*|md*|zram*|mtdblock*) return 0 ;;
    esac
    return 1
}

is_source_disk_name_mock() {
    if [ -n "$ISO_SOURCE_DISK" ] && [ "$1" = "$ISO_SOURCE_DISK" ]; then
        return 0
    fi
    if [ -n "$VENTOY_USB_DISK" ] && [ "$1" = "$VENTOY_USB_DISK" ]; then
        return 0
    fi
    return 1
}

# Redirect /sys/block and /dev checks to our mock tree.
find_target_disk_mock() {
    TARGET_DISK=""
    TARGET_DISK_ERROR=""
    MOCK_SYS="$1"
    ZE_SOURCE="${2:-http}"
    ISO_SOURCE_DISK="${3:-}"
    ZE_TARGET="${4:-}"

    if [ -n "$ZE_TARGET" ]; then
        if ! validate_target_path_mock "$ZE_TARGET"; then
            TARGET_DISK_ERROR="ze.target '$ZE_TARGET' is not a supported whole-disk /dev path"
            return
        fi
        target_name="$(disk_name_from_path_mock "$ZE_TARGET")"
        if [ ! -d "$MOCK_SYS/block/$target_name" ]; then
            TARGET_DISK_ERROR="ze.target '$ZE_TARGET' is not a block device"
            return
        fi
        if is_source_disk_name_mock "$target_name"; then
            TARGET_DISK_ERROR="ze.target '$ZE_TARGET' is the ISO source media"
            return
        fi
        TARGET_DISK="$ZE_TARGET"
        return
    fi

    count=0
    for dev in "$MOCK_SYS"/block/*; do
        [ -d "$dev" ] || continue
        name="$(basename "$dev")"

        if is_skipped_disk_name_mock "$name"; then
            continue
        fi
        if is_source_disk_name_mock "$name"; then
            continue
        fi

        if [ -f "$dev/removable" ]; then
            removable="$(cat "$dev/removable")"
            if [ "$removable" = "1" ]; then
                continue
            fi
        fi

        count=$((count + 1))
        if [ "$count" -eq 1 ]; then
            TARGET_DISK="/dev/$name"
            if [ "$ZE_SOURCE" != "iso" ]; then
                break
            fi
            continue
        fi
        if [ "$ZE_SOURCE" = "iso" ]; then
            TARGET_DISK=""
            TARGET_DISK_ERROR="multiple target disks found; set ze.target=/dev/<disk> on the kernel command line"
            return
        fi
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

# Test 9: QEMU `virt` machine pflash devices are skipped.
mkdir -p "$TMPDIR/test9/block/mtdblock0" "$TMPDIR/test9/block/mtdblock1" "$TMPDIR/test9/block/vda"
echo "0" > "$TMPDIR/test9/block/vda/removable"
find_target_disk_mock "$TMPDIR/test9"
assert_eq "skip-mtdblock" "/dev/vda" "$TARGET_DISK"

# Test 10: ISO source media is never selected as target.
mkdir -p "$TMPDIR/test10/block/sda" "$TMPDIR/test10/block/vda"
echo "0" > "$TMPDIR/test10/block/sda/removable"
echo "0" > "$TMPDIR/test10/block/vda/removable"
find_target_disk_mock "$TMPDIR/test10" iso sda
assert_eq "skip-iso-source" "/dev/vda" "$TARGET_DISK"

# Test 11: ISO mode refuses multiple implicit target disks.
mkdir -p "$TMPDIR/test11/block/vda" "$TMPDIR/test11/block/vdb"
echo "0" > "$TMPDIR/test11/block/vda/removable"
echo "0" > "$TMPDIR/test11/block/vdb/removable"
find_target_disk_mock "$TMPDIR/test11" iso
assert_eq "iso-multiple-target" "" "$TARGET_DISK"
assert_eq "iso-multiple-target-error" "multiple target disks found; set ze.target=/dev/<disk> on the kernel command line" "$TARGET_DISK_ERROR"

# Test 12: HTTP mode keeps the existing first-fixed-disk behavior.
find_target_disk_mock "$TMPDIR/test11" http
assert_eq "http-first-target" "/dev/vda" "$TARGET_DISK"
assert_eq "http-first-target-no-error" "" "$TARGET_DISK_ERROR"

# Test 13: explicit target selects that disk even when multiple candidates exist.
find_target_disk_mock "$TMPDIR/test11" iso "" "/dev/vdb"
assert_eq "explicit-target" "/dev/vdb" "$TARGET_DISK"
assert_eq "explicit-target-no-error" "" "$TARGET_DISK_ERROR"

# Test 14: explicit target cannot be source media.
find_target_disk_mock "$TMPDIR/test11" iso vdb "/dev/vdb"
assert_eq "explicit-source-target" "" "$TARGET_DISK"
assert_eq "explicit-source-target-error" "ze.target '/dev/vdb' is the ISO source media" "$TARGET_DISK_ERROR"

# Test 15: explicit target must be a whole disk, not a partition.
find_target_disk_mock "$TMPDIR/test11" iso "" "/dev/vdb1"
assert_eq "explicit-partition-target" "" "$TARGET_DISK"
assert_eq "explicit-partition-target-error" "ze.target '/dev/vdb1' is not a supported whole-disk /dev path" "$TARGET_DISK_ERROR"

# Test 16: explicit target validator accepts supported whole-disk names.
for path in /dev/sda /dev/vda /dev/xvda /dev/hda /dev/sdaa /dev/nvme0n1 /dev/nvme12n34 /dev/mmcblk0 /dev/mmcblk10; do
    assert_valid_target_path "valid-target-$path" "$path"
done

# Test 17: explicit target validator rejects partitions and eMMC pseudo-devices.
for path in /dev/sda1 /dev/vda1 /dev/xvda1 /dev/hda1 /dev/nvme0n1p1 /dev/mmcblk0p1 /dev/mmcblk0boot0 /dev/mmcblk0boot1 /dev/mmcblk0rpmb; do
    assert_invalid_target_path "invalid-target-$path" "$path"
done

# Test 18: eMMC boot/RPMB pseudo-devices are rejected before block-device lookup.
mkdir -p "$TMPDIR/test18/block/mmcblk0boot0" "$TMPDIR/test18/block/mmcblk0boot1" "$TMPDIR/test18/block/mmcblk0rpmb"
for path in /dev/mmcblk0boot0 /dev/mmcblk0boot1 /dev/mmcblk0rpmb; do
    find_target_disk_mock "$TMPDIR/test18" iso "" "$path"
    assert_eq "explicit-emmc-pseudo-target-$path" "" "$TARGET_DISK"
    assert_eq "explicit-emmc-pseudo-target-error-$path" "ze.target '$path' is not a supported whole-disk /dev path" "$TARGET_DISK_ERROR"
done


# Test 19: Ventoy USB disk is excluded from target candidates (loop device
# is the ISO source, physical USB disk is tracked via VENTOY_USB_DISK).
mkdir -p "$TMPDIR/test19/block/sda" "$TMPDIR/test19/block/nvme0n1"
echo "0" > "$TMPDIR/test19/block/sda/removable"
echo "0" > "$TMPDIR/test19/block/nvme0n1/removable"
VENTOY_USB_DISK="sda"
find_target_disk_mock "$TMPDIR/test19" iso loop0
assert_eq "ventoy-usb-excluded" "/dev/nvme0n1" "$TARGET_DISK"
assert_eq "ventoy-usb-excluded-no-error" "" "$TARGET_DISK_ERROR"
VENTOY_USB_DISK=""

assert_eq "multi-digit-sd-source-disk" "sda" "$(disk_name_from_path_mock /dev/sda10)"
echo "---"
echo "disk-detect: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
