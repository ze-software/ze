# A build carrier one minor behind the declaration.
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY . .
RUN go build -o /tool ./cmd/tool
