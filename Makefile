# Proto generation for the example

.PHONY: generate_example

generate_example:
	protoc -I ./example/api \
		--go_out=./example/protogen --go_opt=paths=source_relative --go-grpc_out=./example/protogen \
		--grpc-gateway_out ./example/protogen --grpc-gateway_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		example/api/greeter.proto

