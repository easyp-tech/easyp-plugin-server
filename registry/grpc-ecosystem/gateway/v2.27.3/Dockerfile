FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.22 AS build

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN apk add upx=5.0.2-r0 --no-cache

RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 \
    go install -ldflags="-s -w" -trimpath github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.3 \
 && mv /go/bin/${GOOS}_${GOARCH}/protoc-gen-grpc-gateway /go/bin/protoc-gen-grpc-gateway || true \
 && upx --best --lzma /go/bin/protoc-gen-grpc-gateway

FROM scratch

COPY --from=build --link --chown=root:root /etc/passwd /etc/passwd
COPY --from=build --link --chown=root:root /go/bin/protoc-gen-grpc-gateway /protoc-gen-grpc-gateway

USER nobody

ENTRYPOINT [ "/protoc-gen-grpc-gateway" ]
