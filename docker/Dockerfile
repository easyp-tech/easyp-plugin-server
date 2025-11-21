FROM golang:alpine3.22 AS builder

RUN apk update && apk add --no-cache ca-certificates

COPY . /app

WORKDIR /app

RUN go build -o back ./cmd/main.go

FROM alpine:3.22

RUN apk add --no-cache docker-cli ca-certificates

COPY --from=builder /app/back /back
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/back"]
