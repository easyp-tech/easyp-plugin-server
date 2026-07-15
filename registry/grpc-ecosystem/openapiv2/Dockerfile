# syntax=docker/dockerfile:1.19
FROM golang:1.26-alpine3.22 AS build

ARG VERSION
ARG GO_MODULE
ARG BINARY_NAME=protoc-gen-openapiv2
RUN apk add upx --no-cache
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go install -ldflags "-s -w" -trimpath ${GO_MODULE}@${VERSION}
RUN cp /go/bin/${BINARY_NAME} /plugin 2>/dev/null \
    || cp /go/bin/linux_*/${BINARY_NAME} /plugin \
    && upx --best --lzma /plugin

FROM scratch
COPY --from=build /plugin /plugin
