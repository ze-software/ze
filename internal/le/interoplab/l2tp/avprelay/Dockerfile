FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY internal/le/interoplab/l2tp/avprelay/main.go internal/le/interoplab/l2tp/avprelay/pppinject.go ./
RUN CGO_ENABLED=0 go build -o /avp-relay ./main.go ./pppinject.go

FROM alpine:3.24.3
COPY --from=builder /avp-relay /usr/local/bin/avp-relay
ENTRYPOINT ["/usr/local/bin/avp-relay"]
