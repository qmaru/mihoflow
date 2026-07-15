FROM node:24-alpine AS build-ui

WORKDIR /usr/src

RUN corepack enable

COPY ui/package.json ui/pnpm-lock.yaml ./

RUN pnpm install --frozen-lockfile --config.minimum-release-age=0

COPY ui/ ./

RUN pnpm config set minimumReleaseAge 0 && pnpm build

FROM golang:1.26-alpine AS build-server

WORKDIR /usr/src

RUN apk add --no-cache upx ca-certificates tzdata build-base

COPY go.mod go.sum .

RUN go mod download

COPY --from=build-ui /usr/src/dist ./ui/dist
COPY services ./services
COPY main.go .

RUN CGO_ENABLED=1 go build -ldflags="-s -w -linkmode external -extldflags '-static'" -o /usr/src/bin/mihoflow \
    && upx --best --lzma /usr/src/bin/mihoflow

FROM scratch

WORKDIR /

COPY --from=build-server /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-server /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build-server /usr/src/bin/mihoflow /mihoflow

ENTRYPOINT ["/mihoflow"]
