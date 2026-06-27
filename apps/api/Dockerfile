# syntax=docker/dockerfile:1.7

FROM golang:1.26.4-bookworm AS build

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY apps/api/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/talentpilot-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

ENV APP_ENV=production
ENV API_ADDR=:8080

COPY --from=build /out/talentpilot-api /usr/local/bin/talentpilot-api

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/talentpilot-api"]
