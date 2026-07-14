FROM golang:1.26-alpine AS build

WORKDIR /usr/src

RUN apk add --no-cache upx ca-certificates tzdata build-base

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-s -w -linkmode external -extldflags '-static'" -o /usr/src/bin/mihoflow \
    && upx --best --lzma /usr/src/bin/mihoflow

FROM scratch

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /usr/src/bin/mihoflow /mihoflow

ENTRYPOINT ["/mihoflow"]
