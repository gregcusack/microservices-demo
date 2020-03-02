FROM golang:1.14-alpine AS builder
RUN apk add --no-cache ca-certificates git && \
    wget -qO/go/bin/dep https://github.com/golang/dep/releases/download/v0.5.4/dep-linux-amd64 && \
    chmod +x /go/bin/dep

ENV PROJECT github.com/triplewy/microservices-demo/src/recommendationservice
WORKDIR /go/src/$PROJECT

# restore dependencies
COPY Gopkg.* ./
RUN dep ensure --vendor-only -v

COPY . .
RUN go build -o /recommendationservice .

FROM alpine AS release
RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.2.0 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

WORKDIR /recommendationservice
COPY --from=builder /recommendationservice ./server

ENV PORT "8080"
EXPOSE 8080

ENTRYPOINT ["/recommendationservice/server"]

