# syntax=docker/dockerfile:1.18
FROM debian:bookworm-20250908 AS build
ARG VERSION

ARG TARGETARCH
ARG BAZEL_STARTUP_OPTS="--host_jvm_args=-Djava.net.preferIPv4Stack=true"
ARG BAZEL_JOBS=4
ARG BAZEL_BUILD_OPTS="--spawn_strategy=local --jobs=${BAZEL_JOBS} --features=-layering_check --host_features=-layering_check --disk_cache=/build/.cache/bazel-disk --repository_cache=/build/.cache/bazel-repo"
ENV USE_BAZEL_VERSION=7.6.1

RUN apt-get update \
 && apt-get install -y curl git cmake build-essential autoconf clang libc++-dev libtool pkg-config unzip zip
RUN curl -fsSL -o /usr/local/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/download/v1.27.0/bazelisk-linux-${TARGETARCH} \
 && chmod +x /usr/local/bin/bazelisk \
 && mkdir -p /build/.cache \
 && chown -R nobody:nogroup /build \
 && usermod --home /build nobody

USER nobody
WORKDIR /build
RUN curl -fsSL -o grpc-web-src.tar.gz https://github.com/grpc/grpc-web/releases/download/${VERSION#v}/grpc-web-source-${VERSION#v}.tar.gz \
 && tar -zxf grpc-web-src.tar.gz \
 && rm grpc-web-src.tar.gz
WORKDIR /build/grpc-web-${VERSION#v}
RUN --mount=type=cache,id=bazel-grpc-disk,target=/build/.cache/bazel-disk,uid=65534,gid=65534,sharing=shared \
    --mount=type=cache,id=bazel-grpc-repo,target=/build/.cache/bazel-repo,uid=65534,gid=65534,sharing=shared \
    --mount=type=cache,id=bazelisk-grpc,target=/build/.cache/bazelisk,uid=65534,gid=65534,sharing=shared \
    bazelisk ${BAZEL_STARTUP_OPTS} build ${BAZEL_BUILD_OPTS} '//javascript/net/grpc/web/generator:protoc-gen-grpc-web'

FROM gcr.io/distroless/cc-debian12:latest@sha256:620d8b11ae800f0dbd7995f89ddc5344ad603269ea98770588b1b07a4a0a6872 AS base
ARG VERSION

FROM scratch
ARG VERSION
COPY --from=base --link / /
COPY --from=build --link --chmod=0755 /build/grpc-web-${VERSION#v}/bazel-bin/javascript/net/grpc/web/generator/protoc-gen-grpc-web /plugin
USER nobody
ENTRYPOINT ["/plugin"]
