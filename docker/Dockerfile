# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG ZE_VERSION
ARG ZE_BUILD_DATE
ARG ZE_TAGS

RUN CGO_ENABLED=0 go build \
    ${ZE_TAGS:+-tags ${ZE_TAGS}} \
    -ldflags "-X main.version=${ZE_VERSION:-dev} -X main.buildDate=${ZE_BUILD_DATE:-unknown}" \
    -o /ze ./cmd/ze

# Runtime stage
FROM scratch

COPY --from=builder /ze /ze

EXPOSE 179 1790 8080

ENTRYPOINT ["/ze"]
