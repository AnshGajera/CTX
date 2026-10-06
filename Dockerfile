FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/AnshGajera/CTX/internal/version.Version=${VERSION} -X github.com/AnshGajera/CTX/internal/version.Commit=${COMMIT} -X github.com/AnshGajera/CTX/internal/version.Date=${DATE}" -o /ctx ./cmd/ctx

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git
COPY --from=builder /ctx /usr/local/bin/ctx
ENTRYPOINT ["ctx"]
CMD ["status"]
