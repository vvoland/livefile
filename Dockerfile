FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS base

# devcontainer
FROM base AS gopls
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
        GOBIN=/build/ GO111MODULE=on go install "golang.org/x/tools/gopls@latest" \
     && /build/gopls version

FROM base AS devcontainer
COPY --from=gopls /build/gopls /usr/local/bin/gopls

# deps
FROM base AS deps

WORKDIR /go/src

RUN --mount=type=bind,src=./go.mod,target=./go.mod \
    --mount=type=bind,src=./go.sum,target=./go.sum \
    go mod download

# test
FROM deps AS test

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=bind,target=/go/src \
    --mount=type=cache,target=/root/.cache/go-build \
        go test -v
