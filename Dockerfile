FROM golang:1.26-bookworm AS builder

COPY go.mod go.mod

RUN go mod download

COPY . /app

WORKDIR /app

RUN go build -o easyp ./cmd/easyp/

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/easyp /easyp

VOLUME ["/plugins"]

ENTRYPOINT ["/easyp"]
