FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.1.5
RUN CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/AnshGajera/CTX/server/internal/version.Version=${VERSION}" -o ctx-server ./cmd/ctx-server

FROM alpine:3.20
RUN apk add --no-cache sqlite-libs ca-certificates wget
COPY --from=builder /app/ctx-server /usr/local/bin/ctx-server

EXPOSE 3200
VOLUME ["/data"]

ENV CTX_DB=/data/ctx-server.db
ENV CTX_BIND=0.0.0.0
ENV CTX_PORT=3200

ENTRYPOINT ["ctx-server"]
