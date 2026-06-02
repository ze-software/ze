#!/bin/sh
# Unit tests for ISO media discovery logic in the init script.

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

find_iso_media_mock() {
    MOCK_SYS="$1"
    MOCK_MEDIA="$2"
    ZE_IMAGE="$3"
    ZE_MEDIA_ID="$4"
    ISO_SOURCE_DEV=""
    ISO_SOURCE_DISK=""

    for dev in "$MOCK_SYS"/block/*; do
        [ -d "$dev" ] || continue
        name="$(basename "$dev")"
        for media in "$MOCK_MEDIA"/"$name" "$MOCK_MEDIA"/"$name"[0-9]* "$MOCK_MEDIA"/"$name"p[0-9]*; do
            [ -d "$media" ] || continue
            node_name="$(basename "$media")"
            media_id_path="$media/ze-install/media-id"
            if [ -f "$media/ze-install/manifest.json" ] && [ -f "$media/ze-install/images/$ZE_IMAGE" ] && [ -f "$media_id_path" ]; then
                candidate_media_id="$(cat "$media_id_path")"
                if [ "$candidate_media_id" = "$ZE_MEDIA_ID" ]; then
                    ISO_SOURCE_DEV="/dev/$node_name"
                    ISO_SOURCE_DISK="$(disk_name_from_path_mock "/dev/$node_name")"
                    return 0
                fi
            fi
        done
    done
    return 1
}

make_media() {
    root="$1"
    node="$2"
    image="$3"
    media_id="$4"
    mkdir -p "$root/$node/ze-install/images"
    : > "$root/$node/ze-install/manifest.json"
    printf '%s\n' "$media_id" > "$root/$node/ze-install/media-id"
    : > "$root/$node/ze-install/images/$image"
}

mkdir -p "$TMPDIR/test1/sys/block/sr0" "$TMPDIR/test1/media"
make_media "$TMPDIR/test1/media" sr0 ze.img 0123456789abcdef0123456789abcdef
find_iso_media_mock "$TMPDIR/test1/sys" "$TMPDIR/test1/media" ze.img 0123456789abcdef0123456789abcdef
assert_eq "find-sr0-dev" "/dev/sr0" "$ISO_SOURCE_DEV"
assert_eq "find-sr0-disk" "sr0" "$ISO_SOURCE_DISK"

mkdir -p "$TMPDIR/test2/sys/block/sr0" "$TMPDIR/test2/media/sr0/ze-install/images"
printf '%s\n' 0123456789abcdef0123456789abcdef > "$TMPDIR/test2/media/sr0/ze-install/media-id"
: > "$TMPDIR/test2/media/sr0/ze-install/images/ze.img"
if find_iso_media_mock "$TMPDIR/test2/sys" "$TMPDIR/test2/media" ze.img 0123456789abcdef0123456789abcdef; then
    assert_eq "reject-no-manifest" "fail" "ok"
else
    assert_eq "reject-no-manifest" "fail" "fail"
fi

mkdir -p "$TMPDIR/test3/sys/block/sdb" "$TMPDIR/test3/media"
make_media "$TMPDIR/test3/media" sdb1 ze-special.img fedcba9876543210fedcba9876543210
find_iso_media_mock "$TMPDIR/test3/sys" "$TMPDIR/test3/media" ze-special.img fedcba9876543210fedcba9876543210
assert_eq "partition-source-dev" "/dev/sdb1" "$ISO_SOURCE_DEV"
assert_eq "partition-source-disk" "sdb" "$ISO_SOURCE_DISK"

mkdir -p "$TMPDIR/test3b/sys/block/sdb" "$TMPDIR/test3b/media"
make_media "$TMPDIR/test3b/media" sdb10 ze-special.img fedcba9876543210fedcba9876543210
find_iso_media_mock "$TMPDIR/test3b/sys" "$TMPDIR/test3b/media" ze-special.img fedcba9876543210fedcba9876543210
assert_eq "multi-digit-partition-source-dev" "/dev/sdb10" "$ISO_SOURCE_DEV"
assert_eq "multi-digit-partition-source-disk" "sdb" "$ISO_SOURCE_DISK"

mkdir -p "$TMPDIR/test4/sys/block/sr0" "$TMPDIR/test4/media"
if find_iso_media_mock "$TMPDIR/test4/sys" "$TMPDIR/test4/media" ze.img 0123456789abcdef0123456789abcdef; then
    assert_eq "no-media" "fail" "ok"
else
    assert_eq "no-media" "fail" "fail"
fi

mkdir -p "$TMPDIR/test5/sys/block/sr0" "$TMPDIR/test5/media"
make_media "$TMPDIR/test5/media" sr0 ze.img 0123456789abcdef0123456789abcdef
if find_iso_media_mock "$TMPDIR/test5/sys" "$TMPDIR/test5/media" ze.img fedcba9876543210fedcba9876543210; then
    assert_eq "media-id-mismatch" "fail" "ok"
else
    assert_eq "media-id-mismatch" "fail" "fail"
fi

mkdir -p "$TMPDIR/test6/sys/block/sr0" "$TMPDIR/test6/sys/block/sr1" "$TMPDIR/test6/media"
make_media "$TMPDIR/test6/media" sr0 ze.img 0123456789abcdef0123456789abcdef
make_media "$TMPDIR/test6/media" sr1 ze.img fedcba9876543210fedcba9876543210
find_iso_media_mock "$TMPDIR/test6/sys" "$TMPDIR/test6/media" ze.img fedcba9876543210fedcba9876543210
assert_eq "matching-media-id-dev" "/dev/sr1" "$ISO_SOURCE_DEV"
assert_eq "matching-media-id-disk" "sr1" "$ISO_SOURCE_DISK"

echo "---"
echo "iso-media: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
