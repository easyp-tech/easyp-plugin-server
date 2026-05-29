# syntax=docker/dockerfile:1.4
FROM golang:1.20.3-bullseye AS build
ARG VERSION
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 \
    go install -ldflags "-s -w" -trimpath github.com/chrusty/protoc-gen-jsonschema/cmd/protoc-gen-jsonschema@${VERSION#v}

FROM scratch
ARG VERSION
COPY --from=build --link --chown=root:root /etc/passwd /etc/passwd
COPY --from=build --link --chown=root:root /go/bin/protoc-gen-jsonschema /
USER nobody
ENTRYPOINT [ "/protoc-gen-jsonschema" ]
