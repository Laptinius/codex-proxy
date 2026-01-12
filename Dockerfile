# Multi-stage Dockerfile for Go proxy

FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/codex-proxy ./cmd/codex-proxy

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/codex-proxy /app/codex-proxy

EXPOSE 8787
ENV ADDR=0.0.0.0:8787

ENTRYPOINT ["/app/codex-proxy"]
