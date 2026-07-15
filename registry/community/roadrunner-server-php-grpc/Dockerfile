# syntax=docker/dockerfile:1.22
FROM --platform=$BUILDPLATFORM golang:1.26.1-trixie@sha256:ce3f1c8d3718a306811d8d5e547073b466b15e85bfa7e1b4f0dc45516c95b72d AS build
ARG VERSION

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH

RUN git clone --depth=1 --branch ${VERSION} https://github.com/roadrunner-server/grpc.git
RUN --mount=type=cache,target=/go/pkg/mod \
    cd grpc/protoc_plugins/protoc-gen-php-grpc \
 && go install -ldflags="-s -w" -trimpath \
 && mv /go/bin/${GOOS}_${GOARCH}/protoc-gen-php-grpc /go/bin/protoc-gen-php-grpc || true

FROM scratch
ARG VERSION
COPY --from=build --link --chown=root:root /etc/passwd /etc/passwd
COPY --from=build --link --chown=root:root /go/bin/protoc-gen-php-grpc /plugin
USER nobody
ENTRYPOINT ["/plugin"]
