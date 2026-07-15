# syntax=docker/dockerfile:1.23
FROM swift:6.3.2-bookworm@sha256:40918972df286d15aa47f4686c98a211f0a6152ca1d48b3dbccd6f56a3c34815 AS build
ARG VERSION
ARG BINARY_NAME=protoc-gen-grpc-swift-2
RUN apt-get update \
 && apt-get install -y unzip
WORKDIR /app
RUN git clone --depth 1 --branch ${VERSION#v} https://github.com/grpc/grpc-swift-protobuf --recursive
WORKDIR /app/grpc-swift-protobuf
COPY --link Package.resolved .
RUN swift build -c release --static-swift-stdlib --product ${BINARY_NAME} -Xlinker -s \
 && cp "/app/grpc-swift-protobuf/.build/release/${BINARY_NAME}" /plugin

FROM gcr.io/distroless/cc-debian13:latest@sha256:8b5d1db6d2253036a53cb8362d3e3fa82a7caf84c247772c46a023166c64e977 AS base
ARG VERSION

FROM scratch
ARG VERSION
COPY --link --from=base / /
COPY --link --from=build /plugin /plugin
USER nobody
ENTRYPOINT ["/plugin"]
