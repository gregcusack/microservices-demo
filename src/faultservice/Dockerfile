# Builder stage
FROM golang:1.14-alpine AS builder

RUN apk add --no-cache ca-certificates git && \
    wget -qO/go/bin/dep https://github.com/golang/dep/releases/download/v0.5.4/dep-linux-amd64 && \
    chmod +x /go/bin/dep

ENV SERVICE faultservice

WORKDIR /go/src/github.com/triplewy/microservices-demo/src/${SERVICE}

COPY . .

RUN go build -o /${SERVICE} ./cmd/server

RUN mv ./cmd/server/data /data

# Release stage
FROM alpine AS release

ENV SERVICE faultservice

RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

WORKDIR /${SERVICE}

COPY --from=builder /${SERVICE} ./server
COPY --from=builder /data ./data

EXPOSE 8080

ENTRYPOINT ["./server"]

