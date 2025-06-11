package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"github.com/eliziario/simpledb-mcp/internal/database"
	"github.com/eliziario/simpledb-mcp/internal/tools"
	"github.com/gin-gonic/gin"
	"github.com/metoro-io/mcp-golang"
	httpTransport "github.com/metoro-io/mcp-golang/transport/http"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type Server struct {
	config      *config.Config
	dbManager   *database.Manager
	credManager *credentials.Manager
	toolHandler *tools.Handler
	mcpServer   *mcp_golang.Server
	httpServer  *http.Server
	ginEngine   *gin.Engine
}

func NewServer() (*Server, error) {
	return NewServerWithFlags("", "", "")
}

func NewServerWithFlags(transport, address, path string) (*Server, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags if provided
	if transport != "" {
		cfg.Settings.Server.Transport = transport
	}
	if address != "" {
		cfg.Settings.Server.Address = address
	}
	if path != "" {
		cfg.Settings.Server.Path = path
	}

	// Initialize credential manager
	credManager := credentials.NewManager(cfg.Settings.CacheCredentials)

	// Initialize database manager
	dbManager := database.NewManager(cfg, credManager)

	// Create MCP server with selected transport
	var mcpServer *mcp_golang.Server
	var httpServer *http.Server
	var ginEngine *gin.Engine

	switch cfg.Settings.Server.Transport {
	case "stdio":
		transport := stdio.NewStdioServerTransport()
		mcpServer = mcp_golang.NewServer(transport)
	case "http":
		transport := httpTransport.NewHTTPTransport(cfg.Settings.Server.Path).WithAddr(cfg.Settings.Server.Address)
		mcpServer = mcp_golang.NewServer(transport)
	case "gin":
		transport := httpTransport.NewGinTransport()
		mcpServer = mcp_golang.NewServer(transport)

		gin.SetMode(gin.ReleaseMode)
		ginEngine = gin.New()
		ginEngine.Use(gin.Logger(), gin.Recovery())
		ginEngine.POST(cfg.Settings.Server.Path, transport.Handler())

		httpServer = &http.Server{
			Addr:    cfg.Settings.Server.Address,
			Handler: ginEngine,
		}
	default:
		return nil, fmt.Errorf("unsupported transport: %s", cfg.Settings.Server.Transport)
	}

	server := &Server{
		config:      cfg,
		dbManager:   dbManager,
		credManager: credManager,
		mcpServer:   mcpServer,
		httpServer:  httpServer,
		ginEngine:   ginEngine,
	}

	// Initialize tool handler with the server
	toolHandler, err := tools.NewHandler(dbManager, cfg, mcpServer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tool handler: %w", err)
	}
	server.toolHandler = toolHandler

	return server, nil
}

func (s *Server) Run(ctx context.Context) error {
	log.Printf("Starting SimpleDB MCP Server v0.1.0")
	log.Printf("Configuration loaded with %d connections", len(s.config.Connections))
	log.Printf("Using %s transport", s.config.Settings.Server.Transport)

	errChan := make(chan error, 1)

	switch s.config.Settings.Server.Transport {
	case "stdio":
		// Start the MCP server in a goroutine for stdio
		go func() {
			log.Println("Starting MCP server with stdio transport...")
			err := s.mcpServer.Serve()
			log.Printf("MCP server stopped with error: %v", err)
			errChan <- err
		}()

	case "http":
		// HTTP transport handles its own server lifecycle
		go func() {
			log.Printf("Starting MCP server with HTTP transport on %s%s", s.config.Settings.Server.Address, s.config.Settings.Server.Path)
			err := s.mcpServer.Serve()
			log.Printf("MCP server stopped with error: %v", err)
			errChan <- err
		}()

	case "gin":
		// Start HTTP server for Gin transport
		go func() {
			log.Printf("Starting MCP server with Gin transport on %s%s", s.config.Settings.Server.Address, s.config.Settings.Server.Path)
			if err := s.httpServer.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					errChan <- nil
				} else {
					log.Printf("HTTP server error: %v", err)
					errChan <- err
				}
				return
			}
			errChan <- nil
		}()

		// Also start MCP server (for tool registration, etc.)
		go func() {
			// For gin transport, we don't call Serve() as the HTTP server handles requests
			<-ctx.Done()
		}()
	}

	// Wait for either context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("Shutting down server...")
		if s.httpServer != nil {
			if err := s.httpServer.Shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down HTTP server: %v", err)
			}
		}
		return ctx.Err()
	case err := <-errChan:
		log.Printf("Server error received: %v", err)
		return err
	}
}

func (s *Server) Close() error {
	if err := s.dbManager.Close(); err != nil {
		return fmt.Errorf("failed to close database connections: %w", err)
	}

	s.credManager.ClearCache()
	return nil
}

// GetInfo returns server information for debugging
func (s *Server) GetInfo() map[string]interface{} {
	connections := make([]map[string]interface{}, 0, len(s.config.Connections))
	for name, conn := range s.config.Connections {
		status := "unknown"
		if err := s.dbManager.TestConnection(name); err == nil {
			status = "connected"
		} else {
			status = "disconnected"
		}

		connections = append(connections, map[string]interface{}{
			"name":     name,
			"type":     conn.Type,
			"host":     conn.Host,
			"port":     conn.Port,
			"database": conn.Database,
			"status":   status,
		})
	}

	return map[string]interface{}{
		"server": map[string]interface{}{
			"name":    "simpledb-mcp",
			"version": "0.1.0",
		},
		"connections": connections,
		"settings": map[string]interface{}{
			"query_timeout":     s.config.Settings.QueryTimeout.String(),
			"max_rows":          s.config.Settings.MaxRows,
			"cache_credentials": s.config.Settings.CacheCredentials.String(),
			"require_biometric": s.config.Settings.RequireBiometric,
		},
	}
}
