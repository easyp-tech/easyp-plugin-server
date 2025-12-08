FROM golang-1.25:alpine3.22 AS builder

RUN apk update && apk add --no-cache ca-certificates

COPY go.mod go.mod

RUN go mod download

COPY . /app

WORKDIR /app

RUN go build -o easyp ./cmd/main.go

FROM alpine:3.22

RUN apk add --no-cache docker-cli ca-certificates

COPY --from=builder /app/easyp /easyp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/easyp"]
