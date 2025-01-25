package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	api "github.com/n-r-w/grpcsrv/example/protogen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Connect to the gRPC server
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := api.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Test unary call
	resp, err := client.SayHello(ctx, &api.HelloRequest{Name: "World"})
	if err != nil {
		log.Fatalf("SayHello failed: %v", err)
	}
	fmt.Printf("Unary Response: %s (at %s)\n", resp.Message, resp.Timestamp)

	// Test streaming call
	stream, err := client.SayManyHellos(ctx, &api.HelloRequest{Name: "Streaming World"})
	if err != nil {
		log.Fatalf("SayManyHellos failed: %v", err)
	}

	fmt.Println("\nStreaming responses:")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to receive: %v", err)
		}
		fmt.Printf("Stream Response: %s (at %s)\n", resp.Message, resp.Timestamp)
	}
}
