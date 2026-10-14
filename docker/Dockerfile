# Deployment image: a static ze on a scratch base.
#
# For a lab under netlab or containerlab, build docker/Dockerfile.lab instead:
# those tools exec sh, ip and cat inside a node, and this image has none of them.

# Build stage
FROM golang:1.27-alpine AS builder

# git applies Ze's standard-library patch when the build below generates the
# patched GOROOT (docs/contributing/patching-the-standard-library.md).
RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG ZE_VERSION
ARG ZE_BUILD_DATE
# ZE_TAGS carries EXTRA tags (e.g. maprib), the same meaning it has everywhere
# else in the build system. It is added to the default feature set, never a
# replacement for it.
ARG ZE_TAGS

# The personality tag alone is NOT enough: every compile-out-able feature is off
# unless its tag is passed, and BGP itself is one of them (feature-gates.txt,
# ze_bgp). An untagged build has no daemon personality at all, so `ze start` is
# an unknown command, and a `-tags ze_core` build rejects the `bgp` config root
# as an unknown top-level keyword.
#
# The default-on tags are DERIVED from feature-gates.txt through the one shell
# reader of it, feature-tags, so this image can never drift from the shipped
# binary's feature set. Deriving here rather than taking the whole list as a
# build arg keeps `docker compose build` (docker/compose.yaml passes no build
# args) and a bare `docker build` correct.
#
# ze compiles only against Ze's patched crypto/tls, so the same RUN first
# generates the patched GOROOT from the image's go (ENV cannot hold a command's
# output) and builds with its go, GOROOT and GOTOOLCHAIN=local
# (docs/contributing/patching-the-standard-library.md). cache/ is in
# .dockerignore, so the tree lands in a real cache/stdpatch/ inside this stage.
RUN . ./feature-tags && ZE_FEATURES="$(feature_tags feature-gates.txt ' ')" && \
    tree="$(go run ./internal/le/go/stdpatch/goroot)" && \
    GOROOT="$tree" GOTOOLCHAIN=local PATH="$tree/bin:$PATH" CGO_ENABLED=0 "$tree/bin/go" build \
    -tags "ze_core $ZE_FEATURES $ZE_TAGS" \
    -ldflags "-X main.version=${ZE_VERSION:-dev} -X main.buildDate=${ZE_BUILD_DATE:-unknown}" \
    -o /ze ./cmd/ze

# Runtime stage
FROM scratch

COPY --from=builder /ze /ze

EXPOSE 179 1790 8080

ENTRYPOINT ["/ze"]
