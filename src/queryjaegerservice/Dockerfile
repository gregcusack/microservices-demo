FROM golang:1.14-alpine AS builder

RUN apk add --no-cache ca-certificates git && \
    wget -qO/go/bin/dep https://github.com/golang/dep/releases/download/v0.5.4/dep-linux-amd64 && \
    chmod +x /go/bin/dep

ENV SERVICE queryjaegerservice

WORKDIR /go/src/github.com/triplewy/microservices-demo/src/${SERVICE}

COPY Gopkg.* ./

RUN dep ensure --vendor-only -v

COPY . .

RUN go build -o /${SERVICE} .

FROM alpine AS release

ENV SERVICE queryjaegerservice

RUN apk add --no-cache ca-certificates

WORKDIR /${SERVICE}

COPY --from=builder /${SERVICE} ./server

EXPOSE 8080

ENTRYPOINT ["/queryjaegerservice/server"]

