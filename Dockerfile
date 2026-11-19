FROM node:24-bookworm-slim AS web-build

WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/server /server
COPY config.yaml /app/config.yaml
COPY --from=web-build /web/dist /app/web/dist

USER 65532:65532

EXPOSE 8080
STOPSIGNAL SIGTERM

ENTRYPOINT ["/server"]
