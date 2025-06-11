package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eliziario/simpledb-mcp/pkg/api"
)

func main() {
	// Parse command line flags
	transport := flag.String("transport", "", "Transport type: stdio, http, gin (overrides config)")
	address := flag.String("address", "", "Server address for HTTP/Gin transport (e.g., :8080)")
	path := flag.String("path", "", "Endpoint path for HTTP/Gin transport (e.g., /mcp)")
	flag.Parse()

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Create and start server
	server, err := api.NewServerWithFlags(*transport, *address, *path)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Run server
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
