# ---------------------------------------------------
# Stage 1: Build Web Frontend
# ---------------------------------------------------
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---------------------------------------------------
# Stage 2: Build Go Server
# ---------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS go-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ ./pkg/
COPY server/ ./server/
# Copy compiled web dist into server embedded dir
COPY --from=web-builder /app/server/dist ./server/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o /netradar ./server

# ---------------------------------------------------
# Stage 3: Minimal Secure Production Image (<25MB)
# ---------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=go-builder /netradar /app/netradar

EXPOSE 8899
VOLUME ["/data"]

ENV NETRADAR_LISTEN=":8899" \
    NETRADAR_DB_PATH="/data/sqlite/sqlite.db" \
    NETRADAR_GEOIP_DIR="/data/geoip"

ENTRYPOINT ["/app/netradar"]
