FROM golang:1.14-alpine AS builder

ENV PROJECT github.com/triplewy/microservices-demo/src/paymentservice
WORKDIR /go/src/$PROJECT

COPY go.* ./
RUN go mod download
COPY . .
RUN go install .

FROM alpine AS release
RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

WORKDIR /paymentservice
COPY --from=builder /go/bin/paymentservice /paymentservice/server

ENV PORT "50051"
EXPOSE 50051

ENTRYPOINT ["/paymentservice/server"]

