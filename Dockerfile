FROM golang:alpine3.22 AS builder

ARG LICENSE_PUBLIC_KEY=""

RUN apk update && apk add --no-cache ca-certificates

COPY go.mod go.mod

RUN go mod download

COPY . /app

WORKDIR /app

RUN go build -ldflags "-X main.licensePublicKey=${LICENSE_PUBLIC_KEY}" -o easyp ./cmd/main.go

FROM alpine:3.22

RUN apk add --no-cache docker-cli ca-certificates

COPY --from=builder /app/easyp /easyp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/easyp"]
