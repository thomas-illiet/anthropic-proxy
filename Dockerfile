# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

ARG VERSION=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/thomas-illiet/anthropic-proxy/internal/cli.version=${VERSION}" \
    -o /out/anthropic-proxy \
    .

FROM alpine:3.23

ARG VERSION=dev
ARG SOURCE=https://github.com/thomas-illiet/anthropic-proxy
ARG REVISION=unknown

LABEL org.opencontainers.image.title="anthropic-proxy" \
    org.opencontainers.image.description="Anthropic-compatible proxy for OpenAI-compatible chat completions endpoints" \
    org.opencontainers.image.source="${SOURCE}" \
    org.opencontainers.image.revision="${REVISION}" \
    org.opencontainers.image.version="${VERSION}"

RUN apk add --no-cache ca-certificates \
    && addgroup -S anthropic-proxy \
    && adduser -S -D -H -h /nonexistent -G anthropic-proxy anthropic-proxy

WORKDIR /app
COPY --from=build /out/anthropic-proxy /usr/local/bin/anthropic-proxy

EXPOSE 8787
USER anthropic-proxy

ENTRYPOINT ["anthropic-proxy"]
CMD ["serve"]
