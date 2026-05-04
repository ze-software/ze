#!/usr/bin/env bash
set -euo pipefail

kernel_dir=${KERNEL_DIR:?}
upstream_url=${KERNEL_UPSTREAM_URL:?}
kernel_arch=${KERNEL_ARCH:-amd64}
container=${KERNEL_CONTAINER:-docker}
flavor=${KERNEL_FLAVOR:-vanilla}
rebuild_ver=${KERNEL_REBUILD_VER:-latest}

case "$kernel_arch" in
	amd64)
		linux_arch=amd64
		cross=
		platform_args=(--platform=linux/amd64)
		cross_package=
		image_suffix=amd64
		;;
	arm64)
		linux_arch=arm64
		cross=arm64
		platform_args=()
		cross_package="crossbuild-essential-arm64"
		image_suffix=arm64
		;;
	*)
		printf 'error: unsupported KERNEL_ARCH=%s (expected amd64 or arm64)\n' "$kernel_arch" >&2
		exit 1
		;;
esac

build_dir="$kernel_dir/_build"
test -d "$build_dir" || { printf 'error: %s not found\n' "$build_dir" >&2; exit 1; }
test -f "$build_dir/config.addendum.txt" || { printf 'error: %s/config.addendum.txt not found\n' "$build_dir" >&2; exit 1; }
test -f "$build_dir/upstream-url.txt" || { printf 'error: %s/upstream-url.txt not found\n' "$build_dir" >&2; exit 1; }
test -f "$build_dir/series" || { printf 'error: %s/series not found\n' "$build_dir" >&2; exit 1; }

helper_bin="gokr-rebuild-kernel.linux-${linux_arch}"
helper_gopath="$build_dir/.gokr-gopath-${linux_arch}"

printf -- '--- Building Linux %s gokr-rebuild-kernel helper ---\n' "$linux_arch"
mkdir -p "$helper_gopath"
GOPATH="$helper_gopath" GOOS=linux GOARCH="$linux_arch" go install "github.com/gokrazy/autoupdate/cmd/gokr-rebuild-kernel@${rebuild_ver}"
helper_src="$helper_gopath/bin/linux_${linux_arch}/gokr-rebuild-kernel"
if [[ ! -x "$helper_src" ]]; then
	helper_src="$helper_gopath/bin/gokr-rebuild-kernel"
fi
test -x "$helper_src" || { printf 'error: built helper not found under %s\n' "$helper_gopath/bin" >&2; exit 1; }
cp "$helper_src" "$build_dir/$helper_bin"

dockerfile="$build_dir/Dockerfile.ze"
uid=$(id -u)
gid=$(id -g)
{
	printf '%s\n' 'FROM debian:bookworm'
	printf '\n'
	printf '%s\n' 'RUN apt-get update && apt-get install -y \'
	if [[ -n "$cross_package" ]]; then
		printf '  %s \\\n' "$cross_package"
	fi
	printf '%s\n' '  build-essential bc libssl-dev bison flex libelf-dev ncurses-dev ca-certificates zstd kmod python3'
	printf '\n'
	printf 'COPY %s /usr/bin/gokr-rebuild-kernel\n' "$helper_bin"
	printf 'COPY config.addendum.txt /usr/src/config.addendum.txt\n'
	while IFS= read -r patch; do
		[[ -n "$patch" ]] || continue
		printf 'COPY %s /usr/src/%s\n' "$patch" "$patch"
	done < "$build_dir/series"
	printf '\n'
	printf '%s\n' "RUN echo 'builduser:x:${uid}:${gid}:nobody:/:/bin/sh' >> /etc/passwd && \\"
	printf '    chown -R %s:%s /usr/src\n\n' "$uid" "$gid"
	printf '%s\n' 'USER builduser'
	printf '%s\n' 'WORKDIR /usr/src'
	printf '%s\n' 'ENV GOKRAZY_IN_DOCKER=1'
	printf '%s\n' 'ENTRYPOINT ["/usr/bin/gokr-rebuild-kernel"]'
} > "$dockerfile"

printf -- '--- Building kernel container (%s) ---\n' "$kernel_arch"
(
	cd "$build_dir"
	"$container" build "${platform_args[@]}" --rm=true --tag="gokr-rebuild-kernel-${image_suffix}" -f Dockerfile.ze .
)

printf -- '--- Compiling kernel (%s) ---\n' "$kernel_arch"
abs_build_dir=$(cd "$build_dir" && pwd)
run_args=(run "${platform_args[@]}" --volume "${abs_build_dir}:/tmp/buildresult:Z" --rm "gokr-rebuild-kernel-${image_suffix}" -cross="$cross" -flavor="$flavor" "$upstream_url")
"$container" "${run_args[@]}"

printf -- '--- Installing kernel artifacts (%s) ---\n' "$kernel_arch"
cp "$build_dir/vmlinuz" "$kernel_dir/vmlinuz"

for subdir in build source; do
	for match in "$build_dir"/lib/modules/*/"$subdir"; do
		[[ -e "$match" || -L "$match" ]] || continue
		rm -f "$match"
	done
done

mkdir -p "$kernel_dir/lib"
rm -rf "$kernel_dir/lib/modules"
cp -R "$build_dir/lib/modules" "$kernel_dir/lib/"

if [[ "$kernel_arch" == arm64 ]]; then
	shopt -s nullglob
	rm -f "$kernel_dir"/*.dtb
	cp "$build_dir"/*.dtb "$kernel_dir"/ 2>/dev/null || true
	if [[ -d "$build_dir/overlays" ]]; then
		rm -rf "$kernel_dir/overlays"
		cp -R "$build_dir/overlays" "$kernel_dir/"
	fi
fi
