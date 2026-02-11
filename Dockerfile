FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy library module
COPY go.mod go.sum* ./
COPY rebuf/ rebuf/
COPY utils/ utils/

# Copy server module
COPY server/go.mod server/go.sum server/
RUN cd server && go mod download

# Copy server source
COPY server/ server/

# Build
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -o /rebuf-server ./cmd/server

# Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /rebuf-server /usr/local/bin/rebuf-server

RUN mkdir -p /var/lib/rebuf/wal

EXPOSE 8080

ENTRYPOINT ["rebuf-server"]
