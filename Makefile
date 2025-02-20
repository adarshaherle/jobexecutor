.PHONY: all proto build test docker-run docker-test clean

all: proto build

proto:
	protoc --proto_path=proto --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. proto/job.proto

build:
	mkdir -p bin
	go build -o bin/jobexecutor-server ./cmd/server
	go build -o bin/jobexecutor ./cmd/cli

test:
	go test -v -race ./internal/job/...
	go test -v ./internal/grpcserver/...

docker-run:
	docker build --no-cache -t jobexecutor .
	# This target runs the final runtime image.
	docker run --rm -it --privileged --cap-add=ALL --security-opt seccomp=seccomp.json   -v $(PWD)/certs:/app/certs -w /app -p 50051:50051 jobexecutor ./jobexecutor-server --cert=certs/server.crt --key=certs/server.key --ca=certs/ca.crt --addr=:50051

docker-test:
	docker build --no-cache --target tester -t jobexecutor-tester .
	docker run --rm -it --privileged --cap-add=ALL --security-opt seccomp=seccomp.json -e CGROUP_BASE=/tmp/cgroup -e DISABLE_CGROUP=false jobexecutor-tester
clean:
	rm -rf bin
