# Stage 1: Build UI
FROM oven/bun:latest AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/bun.lock ./
RUN bun install --frozen-lockfile
COPY ui/ .
RUN bunx vite build

# Stage 2: Build Go binary
FROM golang:alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist ./internal/api/ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /pintle ./cmd/pintle

# Stage 3: Runtime
FROM scratch
COPY --from=go-builder /pintle /pintle
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 9443 9080
ENTRYPOINT ["/pintle"]
