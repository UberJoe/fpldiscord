# --- stage 1: build the SPA ---
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npx vite build --outDir /web-dist --emptyOutDir

# --- stage 2: build the Go binary ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web-dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /fpldiscord ./cmd/fpldiscord

# --- stage 3: runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /fpldiscord /fpldiscord
EXPOSE 8080
ENTRYPOINT ["/fpldiscord"]
