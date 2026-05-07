.PHONY: all build core core-pro agent frontend run-core run-agent run-frontend tidy clean test

all: build

build: core agent frontend

core:
	cd core && go build -o ../bin/wirepanel-core ./cmd/wirepanel-core

core-pro:
	cd core && go build -tags pro -o ../bin/wirepanel-core-pro ./cmd/wirepanel-core

agent:
	cd agent && go build -o ../bin/wirepanel-agent ./cmd/wirepanel-agent

frontend:
	cd frontend && bun install --frozen-lockfile && bun run build

tidy:
	cd shared && go mod tidy
	cd core && go mod tidy
	cd agent && go mod tidy

run-core:
	cd core && go run ./cmd/wirepanel-core

run-agent:
	cd agent && go run ./cmd/wirepanel-agent

run-frontend:
	cd frontend && bun run dev

test:
	cd core && go test ./...
	cd agent && go test ./...

clean:
	rm -rf bin frontend/dist frontend/node_modules
