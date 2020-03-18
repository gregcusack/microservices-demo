# Builder stage
FROM golang:1.14-alpine AS builder

WORKDIR /service

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o /go/bin/server .

# Release stage
FROM alpine AS release

RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

COPY --from=builder /go/bin/server /go/bin/server

EXPOSE 8080

ENTRYPOINT ["/go/bin/server"]

