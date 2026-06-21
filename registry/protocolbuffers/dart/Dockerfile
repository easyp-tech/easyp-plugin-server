# syntax=docker/dockerfile:1.19
FROM dart:3.10.1-sdk AS build
ARG VERSION

WORKDIR /build
RUN git clone --depth 1 --branch protoc_plugin-${VERSION} https://github.com/google/protobuf.dart.git \
 && cd protobuf.dart/protoc_plugin \
 && dart pub get \
 && dart compile exe bin/protoc_plugin.dart -o /build/protoc-gen-dart

FROM scratch
ARG VERSION
COPY --from=build --link /etc/passwd /etc/passwd
COPY --from=build --link /runtime/ /
COPY --from=build --link /build/protoc-gen-dart /plugin
USER nobody
ENTRYPOINT ["/plugin"]
