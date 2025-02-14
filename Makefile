.PHONY: build test run docker-build docker-run

build:
	@echo "Building server and CLI..."
	mkdir -p bin
	go build -o bin/jobworker-server ./cmd/server/main.go
	go build -o bin/jobworker-cli ./cmd/cli/main.go

test:
	@echo "Running tests..."
	go test -race -coverprofile=coverage.out ./internal/job/...
	@echo "Test coverage:"
	go tool cover -func=coverage.out | grep total

run: build
	@echo "Starting gRPC server..."
	bin/jobworker-server

docker-build:
	docker build -t jobexecutor:latest .

docker-run:
	docker run --rm -it --privileged --cgroupns=host -p 50051:50051 jobexecutor:latest
