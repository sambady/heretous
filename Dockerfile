FROM golang:1.25-bookworm AS development

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

FROM development AS test

CMD ["sh", "-c", "go vet ./... && go test -race -count=1 -timeout=60s ./..."]

FROM development AS build

RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM scratch AS runtime

COPY --from=development /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/server /server

USER 65532:65532

ENV HTTP_ADDR=:8080

EXPOSE 8080
STOPSIGNAL SIGTERM

ENTRYPOINT ["/server"]
