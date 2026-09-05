# Deployment image: a static ze on a scratch base.
#
# For a lab under netlab or containerlab, build docker/Dockerfile.lab instead:
# those tools exec sh, ip and cat inside a node, and this image has none of them.

# Build stage
FROM golang:1.27-alpine AS builder

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
# The default-on tags are DERIVED from feature-gates.txt, the single source of
# truth Makefile ZE_FEATURES and test/interop/Dockerfile.ze read the same way, so
# this image can never drift from the shipped binary's feature set. Deriving here
# rather than taking the whole list as a build arg keeps `docker compose build`
# (docker/compose.yaml passes no build args) and a bare `docker build` correct.
RUN ZE_FEATURES="$(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" && \
    CGO_ENABLED=0 go build \
    -tags "ze_core $ZE_FEATURES $ZE_TAGS" \
    -ldflags "-X main.version=${ZE_VERSION:-dev} -X main.buildDate=${ZE_BUILD_DATE:-unknown}" \
    -o /ze ./cmd/ze

# Runtime stage
FROM scratch

COPY --from=builder /ze /ze

EXPOSE 179 1790 8080

ENTRYPOINT ["/ze"]
