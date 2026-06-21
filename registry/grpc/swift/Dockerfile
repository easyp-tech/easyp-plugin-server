# syntax=docker/dockerfile:1.22
FROM swift:6.3.0-bookworm@sha256:943bb41624442cef8ccca256f1ff7830ce25209cbad88ebfede0aaef268a8a80 AS build
ARG VERSION

RUN apt-get update \
 && apt-get install -y libstdc++-12-dev
WORKDIR /app
RUN git clone --depth 1 --branch ${VERSION#v} https://github.com/grpc/grpc-swift --recursive
WORKDIR /app/grpc-swift
RUN swift build -c release --static-swift-stdlib --product protoc-gen-grpc-swift -Xlinker -s

FROM gcr.io/distroless/cc-debian13:latest@sha256:e1cc90d06703f5dc30ae869fbfce78fce688f21a97efecd226375233a882e62f AS base
ARG VERSION

FROM scratch
ARG VERSION
COPY --link --from=base / /
COPY --link --from=build /app/grpc-swift/.build/release/protoc-gen-grpc-swift /plugin
USER nobody
ENTRYPOINT ["/plugin"]
