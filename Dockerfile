# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
# Build the static frontend bundle for embedding into the Go server image.
RUN npx tsc -b && npx vite build

FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /src/frontend/dist ./api/internal/server/frontend_dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aidocs-server ./api/cmd/aidocs-server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aidocs ./cli

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/aidocs-server /usr/local/bin/aidocs-server
COPY --from=builder /out/aidocs /usr/local/bin/aidocs
EXPOSE 8080
ENV AIDOCS_ADDR=:8080
CMD ["aidocs-server"]
