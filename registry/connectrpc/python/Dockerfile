# syntax=docker/dockerfile:1.19
FROM python:3.13-alpine AS build

ARG VERSION
ARG PIP_PACKAGE
ARG BINARY_NAME=protoc-gen-connect-python

WORKDIR /app
RUN python -m venv venv \
 && ./venv/bin/pip install --no-cache-dir "${PIP_PACKAGE}==${VERSION#v}" \
 && test -x "./venv/bin/${BINARY_NAME}" \
 && cp "./venv/bin/${BINARY_NAME}" /plugin \
 && chmod 0755 /plugin

FROM scratch
COPY --from=build /plugin /plugin
USER nobody
ENTRYPOINT ["/plugin"]
