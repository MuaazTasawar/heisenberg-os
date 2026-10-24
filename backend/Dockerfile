# ---- build ---------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- run -----------------------------------------------------------------
FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 app
COPY --from=build /out/server /usr/local/bin/server

USER app
ENV PORT=8080 APP_ENV=production
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" >/dev/null || exit 1

CMD ["server"]
