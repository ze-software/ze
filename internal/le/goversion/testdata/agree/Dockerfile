# A build carrier that agrees with the declared minor version.
FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go build -o /tool ./cmd/tool

FROM alpine:3.21
COPY --from=builder /tool /usr/local/bin/tool
