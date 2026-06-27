# syntax=docker/dockerfile:1.7

FROM golang:1.26.4-bookworm AS build

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY apps/api/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/talentpilot-api ./cmd/api

FROM debian:bookworm-slim

ENV APP_ENV=production
ENV API_ADDR=:8080

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 talentpilot \
    && useradd --uid 10001 --gid 10001 --create-home --shell /usr/sbin/nologin talentpilot

COPY --from=build /out/talentpilot-api /usr/local/bin/talentpilot-api

USER talentpilot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/talentpilot-api"]
