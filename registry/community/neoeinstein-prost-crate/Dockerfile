# syntax=docker/dockerfile:1.19
FROM rust:1.91-alpine3.22 AS build

ARG VERSION
ARG CRATE_NAME
ARG BINARY_NAME=protoc-gen-prost-crate
RUN apk add musl-dev upx --no-cache
RUN cargo install ${CRATE_NAME} --version ${VERSION#v} --root /out \
    && upx --best --lzma /out/bin/${BINARY_NAME}

FROM scratch
COPY --from=build /out/bin/${BINARY_NAME} /plugin
