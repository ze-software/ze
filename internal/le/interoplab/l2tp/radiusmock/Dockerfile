FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /radius-mock ./internal/le/interoplab/l2tp/radiusmock

FROM alpine:3.21
COPY --from=builder /radius-mock /usr/local/bin/radius-mock
ENTRYPOINT ["/usr/local/bin/radius-mock"]
