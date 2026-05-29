FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.22 AS build

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN apk add upx=5.0.2-r0 --no-cache

RUN --mount=type=cache,target=/go/pkg/mod \
    go install -ldflags "-s -w" -trimpath google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10 \
 && mv /go/bin/${GOOS}_${GOARCH}/protoc-gen-go /go/bin/protoc-gen-go || true \
 && upx --best --lzma /go/bin/protoc-gen-go

FROM scratch
COPY --from=build /go/bin/protoc-gen-go /plugin
