# syntax=docker/dockerfile:1.23
FROM --platform=$BUILDPLATFORM golang:1.26.3-trixie@sha256:d08bf3ed2bd263088ca8e23fefaf10f1b71769f6932f0a4017ba28d2a5baf001 AS build
ARG VERSION
ARG TARGETOS TARGETARCH

WORKDIR /tmp
RUN git clone --depth 1 --branch cmd/protoc-gen-go-grpc/${VERSION} https://github.com/grpc/grpc-go.git
WORKDIR /tmp/grpc-go
# Inject separate_package support without version-locked patches (grpc-go layout
# differs across v1.2–v1.6; a single git apply does not cover all releases).
COPY apply_separate_package.go /tmp/apply_separate_package.go
RUN go run /tmp/apply_separate_package.go
WORKDIR /tmp/grpc-go/cmd/protoc-gen-go-grpc
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -o protoc-gen-go-grpc -ldflags "-s -w" -trimpath

FROM scratch
ARG VERSION
COPY --from=build --link /etc/passwd /etc/passwd
COPY --from=build --link --chown=root:root /tmp/grpc-go/cmd/protoc-gen-go-grpc/protoc-gen-go-grpc /plugin
USER nobody
ENTRYPOINT ["/plugin"]
