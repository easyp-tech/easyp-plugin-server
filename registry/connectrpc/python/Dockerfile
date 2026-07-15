# syntax=docker/dockerfile:1.19
FROM python:3.13-alpine AS build

ARG VERSION
ARG PIP_PACKAGE

WORKDIR /app
RUN python -m venv venv && \
    ./venv/bin/pip install --no-cache-dir "${PIP_PACKAGE}==${VERSION#v}"

FROM scratch
COPY --from=build /app /app
COPY --chmod=0755 <<"EOF" /plugin
#!/bin/sh
exec /app/venv/bin/protoc-gen-connect-python "$@"
EOF
USER nobody
ENTRYPOINT ["/plugin"]
