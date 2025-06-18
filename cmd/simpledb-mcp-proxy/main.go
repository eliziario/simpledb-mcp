package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func setupLogger() *logrus.Logger {
	logger := logrus.New()

	// Get home directory for log file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}

	// Create logs directory
	logDir := filepath.Join(homeDir, ".config", "simpledb-mcp", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fallback to temp directory
		logDir = "/tmp"
	}

	logFile := filepath.Join(logDir, "simpledb-mcp-proxy.log")

	// Configure lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10,   // megabytes
		MaxBackups: 5,    // number of backups
		MaxAge:     30,   // days
		Compress:   true, // compress old log files
	}

	// Set output to lumberjack
	logger.SetOutput(lumberjackLogger)

	// Set log format
	logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})

	// Set log level
	logger.SetLevel(logrus.InfoLevel)

	return logger
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type Proxy struct {
	serverURL string
	client    *http.Client
	logger    *logrus.Logger
}

func NewProxy(serverURL string, logger *logrus.Logger) *Proxy {
	return &Proxy{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (p *Proxy) forwardRequest(request JSONRPCRequest) (*JSONRPCResponse, error) {
	// Marshal request to JSON
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", p.serverURL, bytes.NewBuffer(requestData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	responseData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode != http.StatusOK {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: map[string]interface{}{
				"code":    -32603,
				"message": fmt.Sprintf("HTTP error %d: %s", httpResp.StatusCode, string(responseData)),
			},
		}, nil
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (p *Proxy) handleStdioLoop(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Set a reasonable deadline for stdin reads
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("stdin read error: %w", err)
				}
				// EOF reached
				return nil
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// Parse JSON-RPC request
			var request JSONRPCRequest
			if err := json.Unmarshal([]byte(line), &request); err != nil {
				// Send error response for invalid JSON
				errorResp := JSONRPCResponse{
					JSONRPC: "2.0",
					Error: map[string]interface{}{
						"code":    -32700,
						"message": "Parse error",
						"data":    err.Error(),
					},
				}

				if respData, err := json.Marshal(errorResp); err == nil {
					fmt.Println(string(respData))
				}
				continue
			}

			// Forward request to HTTP server
			response, err := p.forwardRequest(request)
			if err != nil {
				// Send error response for HTTP failures
				errorResp := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      request.ID,
					Error: map[string]interface{}{
						"code":    -32603,
						"message": "Internal error",
						"data":    err.Error(),
					},
				}
				response = &errorResp
			}

			// Send response back via stdout
			if responseData, err := json.Marshal(response); err == nil {
				fmt.Println(string(responseData))
			} else {
				p.logger.Errorf("Failed to marshal response: %v", err)
			}
		}
	}
}

func main() {
	// Parse command line flags
	serverURL := flag.String("server", "http://localhost:48384/mcp", "MCP server URL to proxy to")
	flag.Parse()

	// Validate server URL
	if *serverURL == "" {
		fmt.Fprintln(os.Stderr, "Server URL is required")
		os.Exit(1)
	}

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup logging
	logger := setupLogger()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Create proxy
	proxy := NewProxy(*serverURL, logger)

	// Log startup info
	logger.Info("SimpleDB MCP Proxy starting")
	logger.Infof("Forwarding stdio requests to: %s", *serverURL)
	logger.Info("Ready for JSON-RPC requests on stdin...")

	// Start stdio loop
	if err := proxy.handleStdioLoop(ctx); err != nil && err != context.Canceled {
		logger.Fatalf("Proxy error: %v", err)
	}

	logger.Info("Proxy stopped")
}
