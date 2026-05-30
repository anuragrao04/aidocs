# syntax=docker/dockerfile:1

# Build on the native runner arch and cross-compile to the target arch, so the
# heavy npm/vite/Go work never runs under QEMU emulation.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
# Build the static frontend bundle for embedding into the Go server image.
RUN npx tsc -b && npx vite build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /src/frontend/dist ./api/internal/server/frontend_dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/aidocs-server ./api/cmd/aidocs-server
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/aidocs ./cli

FROM alpine:3.22
# CA certs are arch-independent data files; copy them from the builder rather
# than running apk under emulation in the target-arch final stage.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=builder /out/aidocs-server /usr/local/bin/aidocs-server
COPY --from=builder /out/aidocs /usr/local/bin/aidocs
EXPOSE 8080
ENV AIDOCS_PORT=8080
CMD ["aidocs-server"]
