FROM golang:1.14-alpine as builder

ENV PROJECT github.com/triplewy/microservices-demo/src/frontend
WORKDIR /go/src/$PROJECT

# restore dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go install .

FROM alpine as release
RUN apk add --no-cache ca-certificates \
    busybox-extras net-tools bind-tools
WORKDIR /frontend
COPY --from=builder /go/bin/frontend /frontend/server
COPY ./templates ./templates
COPY ./static ./static
EXPOSE 8080
ENTRYPOINT ["/frontend/server"]
