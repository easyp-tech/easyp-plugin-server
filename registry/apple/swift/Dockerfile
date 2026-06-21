# syntax=docker/dockerfile:1.23
FROM swift:6.3.2-bookworm@sha256:40918972df286d15aa47f4686c98a211f0a6152ca1d48b3dbccd6f56a3c34815 AS build
ARG VERSION

RUN apt-get update \
 && apt-get install -y libstdc++-12-dev unzip

WORKDIR /app
RUN git clone --depth 1 --branch ${VERSION#v} https://github.com/apple/swift-protobuf --recursive
WORKDIR /app/swift-protobuf

RUN swift build -c release --static-swift-stdlib -Xlinker -s

FROM gcr.io/distroless/cc-debian13:latest@sha256:56aaf20ab2523a346a67c8e8f8e8dabe447447d0788b82284d14ad79cd5f93cc AS base
ARG VERSION

FROM scratch
ARG VERSION
COPY --link --from=base / /
COPY --link --from=build /app/swift-protobuf/.build/release/protoc-gen-swift /plugin
USER nobody
ENTRYPOINT ["/plugin"]
