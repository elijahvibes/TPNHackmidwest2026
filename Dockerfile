# ---- frontend ----
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend ----
FROM golang:1.27-alpine AS api
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/api

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=api /api /app/api
COPY --from=web /web/dist /app/public
COPY deploy/akamai-db-ca.crt /app/akamai-db-ca.crt
ENV PORT=8080 STATIC_DIR=/app/public DATABASE_CA_CERT=/app/akamai-db-ca.crt
EXPOSE 8080
CMD ["/app/api"]
