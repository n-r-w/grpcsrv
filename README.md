# gRPCsrv

A comprehensive Go framework for building production-ready gRPC services with built-in HTTP gateway support, observability, and best practices.

[![Go Reference](https://pkg.go.dev/badge/github.com/n-r-w/grpcsrv.svg)](https://pkg.go.dev/github.com/n-r-w/grpcsrv)
[![Go Report Card](https://goreportcard.com/badge/github.com/n-r-w/grpcsrv)](https://goreportcard.com/report/github.com/n-r-w/grpcsrv)

## Key Features

- 🚀 Simplified gRPC service initialization and configuration
- 🛡️ Middleware support with interceptors
- 🌐 Automatic HTTP/REST gateway via grpc-gateway (optional)
- 💪 Built-in health check endpoints (liveness and readiness probes) (optional)
- 📊 Integrated observability with OpenTelemetry and Prometheus (optional)
- 🔄 Automatic recovery handling (optional)
- 📝 Custom logger (optional)
- 🛡️ <https://github.com/n-r-w/bootstrap> integration for graceful shutdowns (optional)

## Installation

```bash
go get github.com/n-r-w/grpcsrv
```

## Code Generation

### 1. Install Protocol Buffer Compiler (protoc)

[official documentation](https://grpc.io/docs/protoc-installation)

### 2. Install Required Go Protobuf Plugins

```bash
# Install protoc-gen-go (generates Go code from .proto files)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Install protoc-gen-go-grpc (generates gRPC service code)
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install protoc-gen-grpc-gateway (generates HTTP/REST gateway)
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

### 3. Generate Code from Proto Files

The project uses [Task](https://taskfile.dev) for managing project tasks. To generate the code from Protocol Buffer definitions, first install Task according to the [official documentation](https://taskfile.dev/installation).

Then run:

```bash
task protogen
```

## Usage Example

- [protobuf definitions](example/api/greeter.proto)
- [generated code](example/protogen)
- [server example](example/server)
- [grpc client example](example/client/grpc/main.go)
- [http client example](example/client/http/main.go)

## Health Checks

The framework provides built-in health check endpoints:

- `/liveness` - for liveness probe
- `/readiness` - for readiness probe

These can be integrated directly into your HTTP handler tree using the `IHealther` interface.
