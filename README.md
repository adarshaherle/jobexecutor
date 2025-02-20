# Job Executor

Job Executor is a secure, resource-isolated system for executing background jobs. It exposes a gRPC API secured with mutual TLS (mTLS) and provides a CLI tool with a kubectl-like interface.

## Features

- **Secure Execution**: Uses mTLS to authenticate both clients and the server.
- **Resource Isolation**: On Linux, jobs are executed in dedicated cgroups using cgroup v2 to limit CPU, memory, and disk I/O. On macOS, stub implementations allow development without enforcement.
- **gRPC API**: Exposes RPC methods to start, stop, query status, and stream output from jobs.
- **CLI Tool**: Built with Cobra, it supports subcommands (`start`, `stop`, `status`, `logs`) with options such as `--output=json` and `--verbose`.
- **Cross-Platform Development**: Designed to build on macOS for development and run on 64-bit Linux for production.
- **Testing**: Includes unit and integration tests. Use Docker to run tests in a Linux environment.

## Prerequisites

- Go 1.24
- [protoc](https://github.com/protocolbuffers/protobuf/releases) and Go plugins:
  - `protoc-gen-go`
  - `protoc-gen-go-grpc`
- Docker (for testing on Linux)

## Installation

1. **Clone the repository:**

   ```bash
   git clone https://github.com/adarshaherle/jobexecutor.git
   cd jobexecutor
