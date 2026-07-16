ARG NODE_VERSION=24-alpine
ARG GO_VERSION=1.26-alpine

# build ui
FROM node:${NODE_VERSION} AS build-ui

WORKDIR /usr/src

RUN corepack enable

COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --config.minimum-release-age=0

COPY ui/ ./
RUN pnpm config set minimumReleaseAge 0 && pnpm build

# go base
FROM golang:${GO_VERSION} AS build-base

WORKDIR /usr/src

RUN apk add --no-cache upx ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY --from=build-ui /usr/assets/dist ./assets/dist
COPY services ./services
COPY assets ./assets
COPY cmd/app ./cmd/app

# build linux
FROM build-base AS build-linux

RUN go build -ldflags="-s -w" \
    -o /usr/src/bin/mihoflow \
    cmd/app/main.go \
    && upx --best --lzma /usr/src/bin/mihoflow

# build windows
FROM build-base AS build-windows

RUN GOOS=windows GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/mihoflow-windows-amd64.exe \
    cmd/app/main.go \
    && upx --best --lzma /out/mihoflow-windows-amd64.exe

FROM scratch AS export-windows

COPY --from=build-windows /out/mihoflow-windows-amd64.exe /mihoflow-windows-amd64.exe

# build macos
FROM build-base AS build-darwin

RUN GOOS=darwin GOARCH=arm64 \
    go build -ldflags="-s -w" \
    -o /out/mihoflow-darwin-arm64 \
    cmd/app/main.go

FROM scratch AS export-darwin

COPY --from=build-darwin /out/mihoflow-darwin-arm64 /mihoflow-darwin-arm64

# build server
FROM scratch AS server

WORKDIR /

COPY --from=build-linux /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-linux /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build-linux /usr/src/bin/mihoflow /mihoflow

ENTRYPOINT ["/mihoflow"]
