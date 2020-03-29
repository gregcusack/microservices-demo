FROM golang:1.14-alpine as builder

ENV PROJECT github.com/triplewy/microservices-demo/src/cartservice
WORKDIR /go/src/$PROJECT

COPY go.* ./
RUN go mod download

COPY . .

RUN go install .

FROM alpine as release
RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

COPY --from=builder /go/bin/cartservice /cartservice/server

ENTRYPOINT ["/cartservice/server"]
