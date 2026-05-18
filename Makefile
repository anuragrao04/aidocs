SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.6

.PHONY: swagger frontend backend cli build build-frontend build-backend build-cli test clean

swagger:
	$(SWAG) init \
		-g main.go \
		-d api/cmd/aidocs-server,api/internal/server,api/internal/repo \
		-o api/internal/server/docs \
		--parseDependency \
		--parseInternal

frontend:
	cd frontend && npm ci && npm run build

backend: swagger frontend
	go build -o bin/aidocs-server ./api/cmd/aidocs-server

cli:
	go build -o bin/aidocs ./cli

build: backend cli

build-frontend: frontend

build-backend: backend

build-cli: cli

test: swagger frontend
	go test ./...

clean:
	rm -rf bin
