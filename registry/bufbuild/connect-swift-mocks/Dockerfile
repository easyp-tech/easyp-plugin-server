# syntax=docker/dockerfile:1.6
FROM swift:5.8.1-focal AS build
ARG VERSION

WORKDIR /app
RUN git clone --depth 1 --branch ${VERSION#v} https://github.com/bufbuild/connect-swift
WORKDIR /app/connect-swift
RUN swift build -c release --product protoc-gen-connect-swift-mocks --static-swift-stdlib -Xlinker -s

FROM gcr.io/distroless/cc-debian11
ARG VERSION
COPY --from=build --link /app/connect-swift/.build/release/protoc-gen-connect-swift-mocks /plugin
USER nobody
ENTRYPOINT ["/plugin"]
