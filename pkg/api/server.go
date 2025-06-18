package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"github.com/eliziario/simpledb-mcp/internal/database"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	config        *config.Config
	dbManager     *database.Manager
	credManager   *credentials.Manager
	mcpServer     *server.MCPServer
	httpServer    *server.StreamableHTTPServer
	stdHTTPServer *http.Server
}

// Tool argument structures
type ListConnectionsArgs struct{}

type ListDatabasesArgs struct {
	Connection string `json:"connection"`
}

type ListSchemasArgs struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
}

type ListTablesArgs struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
	Schema     string `json:"schema,omitempty"`
}

type DescribeTableArgs struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
	Table      string `json:"table"`
	Schema     string `json:"schema,omitempty"`
}

type ListIndexesArgs struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
	Table      string `json:"table"`
	Schema     string `json:"schema,omitempty"`
}

type GetTableSampleArgs struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
	Table      string `json:"table"`
	Schema     string `json:"schema,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type GetConnectionStatusArgs struct {
	Connection string `json:"connection,omitempty"`
}

type GetPoolMetricsArgs struct{}

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

	// Create MCP server using the new framework
	mcpServer := server.NewMCPServer(
		"simpledb-mcp",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	serverInstance := &Server{
		config:      cfg,
		dbManager:   dbManager,
		credManager: credManager,
		mcpServer:   mcpServer,
	}

	// Create HTTP server if needed
	if cfg.Settings.Server.Transport == "http" {
		httpServer := server.NewStreamableHTTPServer(
			mcpServer,
			server.WithEndpointPath(cfg.Settings.Server.Path),
			server.WithStateLess(true), // Disable sessions for compatibility
		)

		stdHTTPServer := &http.Server{
			Addr:    cfg.Settings.Server.Address,
			Handler: httpServer,
		}

		serverInstance.httpServer = httpServer
		serverInstance.stdHTTPServer = stdHTTPServer
	}

	// Register all tools
	if err := serverInstance.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return serverInstance, nil
}

func (s *Server) registerTools() error {
	// Create and register tools
	s.mcpServer.AddTool(
		mcp.NewTool("list_connections", mcp.WithDescription("List all configured database connections")),
		s.handleListConnections,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_databases",
			mcp.WithDescription("List databases available on a connection"),
			mcp.WithString("connection", mcp.Required()),
		),
		s.handleListDatabases,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_schemas",
			mcp.WithDescription("List schemas in a database (PostgreSQL only)"),
			mcp.WithString("connection", mcp.Required()),
			mcp.WithString("database", mcp.Required()),
		),
		s.handleListSchemas,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_tables",
			mcp.WithDescription("List tables in a database/schema"),
			mcp.WithString("connection", mcp.Required()),
			mcp.WithString("database"),
			mcp.WithString("schema"),
		),
		s.handleListTables,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("describe_table",
			mcp.WithDescription("Get detailed information about a table's structure"),
			mcp.WithString("connection", mcp.Required()),
			mcp.WithString("database", mcp.Required()),
			mcp.WithString("table", mcp.Required()),
			mcp.WithString("schema"),
		),
		s.handleDescribeTable,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("list_indexes",
			mcp.WithDescription("List indexes for a table"),
			mcp.WithString("connection", mcp.Required()),
			mcp.WithString("database", mcp.Required()),
			mcp.WithString("table", mcp.Required()),
			mcp.WithString("schema"),
		),
		s.handleListIndexes,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("get_table_sample",
			mcp.WithDescription("Get a sample of data from a table"),
			mcp.WithString("connection", mcp.Required()),
			mcp.WithString("database", mcp.Required()),
			mcp.WithString("table", mcp.Required()),
			mcp.WithString("schema"),
			mcp.WithNumber("limit"),
		),
		s.handleGetTableSample,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("get_connection_status",
			mcp.WithDescription("Get status of database connections"),
			mcp.WithString("connection"),
		),
		s.handleGetConnectionStatus,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("get_pool_metrics",
			mcp.WithDescription("Get connection pool performance metrics"),
		),
		s.handleGetPoolMetrics,
	)

	return nil
}

func (s *Server) handleListConnections(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connections := make([]map[string]interface{}, 0, len(s.config.Connections))
	for name, conn := range s.config.Connections {
		connections = append(connections, map[string]interface{}{
			"name":     name,
			"type":     conn.Type,
			"host":     conn.Host,
			"port":     conn.Port,
			"database": conn.Database,
		})
	}

	result := map[string]interface{}{
		"connections": connections,
		"count":       len(connections),
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleListDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	conn, exists := s.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	var databases []string
	var err error

	switch conn.Type {
	case "mysql":
		databases, err = s.dbManager.ListDatabasesMySQL(connectionName)
	case "postgres":
		databases, err = s.dbManager.ListDatabasesPostgres(connectionName)
	case "salesforce":
		// Return connection name as the single database
		databases = []string{connectionName}
	case "glue":
		databases, err = s.dbManager.ListDatabasesGlue(connectionName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"databases":  databases,
		"count":      len(databases),
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleListSchemas(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	databaseName := mcp.ParseString(request, "database", "")
	if databaseName == "" {
		return nil, fmt.Errorf("database parameter is required")
	}

	conn, exists := s.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	var schemas []string
	var err error

	switch conn.Type {
	case "postgres":
		schemas, err = s.dbManager.ListSchemasPostgres(connectionName, databaseName)
	case "mysql":
		return nil, fmt.Errorf("MySQL does not support schemas - use list_databases instead")
	case "salesforce":
		schemas, err = s.dbManager.ListSchemasSalesforce(connectionName, databaseName)
	case "glue":
		schemas, err = s.dbManager.ListSchemasGlue(connectionName, databaseName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"database":   databaseName,
		"schemas":    schemas,
		"count":      len(schemas),
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleListTables(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	schema := mcp.ParseString(request, "schema", "")

	conn, exists := s.config.GetConnection(connectionName)

	databaseName := mcp.ParseString(request, "database", "")
	if databaseName == "" {
		if conn.Type == "salesforce" {
			databaseName = connectionName // Use connection name as database name
		} else {
			return nil, fmt.Errorf("database parameter is required")
		}
	}

	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	var tables []database.TableInfo
	var err error

	switch conn.Type {
	case "mysql":
		tables, err = s.dbManager.ListTablesMySQL(connectionName, databaseName)
	case "postgres":
		tables, err = s.dbManager.ListTablesPostgres(connectionName, databaseName, schema)
	case "salesforce":
		tables, err = s.dbManager.ListTablesSalesforce(connectionName)
	case "glue":
		tables, err = s.dbManager.ListTablesGlue(connectionName, databaseName, schema)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"database":   databaseName,
		"schema":     schema,
		"tables":     tables,
		"count":      len(tables),
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleDescribeTable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	databaseName := mcp.ParseString(request, "database", "")

	conn, exists := s.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	if databaseName == "" {
		if conn.Type == "salesforce" {
			databaseName = connectionName // Use connection name as database name
		} else {
			return nil, fmt.Errorf("database parameter is required")
		}
	}

	tableName := mcp.ParseString(request, "table", "")
	if tableName == "" {
		return nil, fmt.Errorf("table parameter is required")
	}

	schema := mcp.ParseString(request, "schema", "")

	var tableInfo []database.ColumnInfo
	var err error

	switch conn.Type {
	case "mysql":
		tableInfo, err = s.dbManager.DescribeTableMySQL(connectionName, databaseName, tableName)
	case "postgres":
		tableInfo, err = s.dbManager.DescribeTablePostgres(connectionName, databaseName, tableName, schema)
	case "salesforce":
		tableInfo, err = s.dbManager.DescribeTableSalesforce(connectionName, tableName)
	case "glue":
		tableInfo, err = s.dbManager.DescribeTableGlue(connectionName, databaseName, tableName, schema)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"database":   databaseName,
		"table":      tableName,
		"schema":     schema,
		"columns":    tableInfo,
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleListIndexes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	databaseName := mcp.ParseString(request, "database", "")
	if databaseName == "" {
		return nil, fmt.Errorf("database parameter is required")
	}

	tableName := mcp.ParseString(request, "table", "")
	if tableName == "" {
		return nil, fmt.Errorf("table parameter is required")
	}

	schema := mcp.ParseString(request, "schema", "")

	conn, exists := s.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	var indexes []database.IndexInfo
	var err error

	switch conn.Type {
	case "mysql":
		indexes, err = s.dbManager.ListIndexesMySQL(connectionName, databaseName, tableName)
	case "postgres":
		indexes, err = s.dbManager.ListIndexesPostgres(connectionName, databaseName, tableName, schema)
	case "salesforce":
		indexes, err = s.dbManager.ListIndexesSalesforce(connectionName, tableName)
	case "glue":
		indexes, err = s.dbManager.ListIndexesGlue(connectionName, databaseName, tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"database":   databaseName,
		"table":      tableName,
		"schema":     schema,
		"indexes":    indexes,
		"count":      len(indexes),
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleGetTableSample(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")
	if connectionName == "" {
		return nil, fmt.Errorf("connection parameter is required")
	}

	databaseName := mcp.ParseString(request, "database", "")
	if databaseName == "" {
		return nil, fmt.Errorf("database parameter is required")
	}

	tableName := mcp.ParseString(request, "table", "")
	if tableName == "" {
		return nil, fmt.Errorf("table parameter is required")
	}

	schema := mcp.ParseString(request, "schema", "")
	limit := mcp.ParseInt(request, "limit", 10)

	// Enforce max limit
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 1
	}

	conn, exists := s.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	var sampleData map[string]interface{}
	var err error

	switch conn.Type {
	case "mysql":
		sampleData, err = s.dbManager.GetTableSampleMySQL(connectionName, databaseName, tableName, limit)
	case "postgres":
		sampleData, err = s.dbManager.GetTableSamplePostgres(connectionName, databaseName, tableName, schema, limit)
	case "salesforce":
		sampleData, err = s.dbManager.GetTableSampleSalesforce(connectionName, tableName, limit)
	case "glue":
		sampleData, err = s.dbManager.GetTableSampleGlue(connectionName, databaseName, tableName, limit)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get table sample: %w", err)
	}

	result := map[string]interface{}{
		"connection": connectionName,
		"database":   databaseName,
		"table":      tableName,
		"schema":     schema,
		"limit":      limit,
		"data":       sampleData,
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleGetConnectionStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connectionName := mcp.ParseString(request, "connection", "")

	var result map[string]interface{}

	if connectionName != "" {
		// Get status for specific connection
		err := s.dbManager.TestConnection(connectionName)
		status := "connected"
		errorMsg := ""
		if err != nil {
			status = "disconnected"
			errorMsg = err.Error()
		}

		result = map[string]interface{}{
			"connection": connectionName,
			"status":     status,
			"error":      errorMsg,
		}
	} else {
		// Get status for all connections
		connections := make(map[string]interface{})
		for name := range s.config.Connections {
			err := s.dbManager.TestConnection(name)
			status := "connected"
			errorMsg := ""
			if err != nil {
				status = "disconnected"
				errorMsg = err.Error()
			}

			connections[name] = map[string]interface{}{
				"status": status,
				"error":  errorMsg,
			}
		}

		result = map[string]interface{}{
			"connections": connections,
		}
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) handleGetPoolMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metrics := s.dbManager.GetPoolMetrics()

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (s *Server) Run(ctx context.Context) error {
	log.Printf("Starting SimpleDB MCP Server v0.1.0")
	log.Printf("Configuration loaded with %d connections", len(s.config.Connections))
	log.Printf("Using %s transport", s.config.Settings.Server.Transport)

	errChan := make(chan error, 1)

	switch s.config.Settings.Server.Transport {
	case "stdio":
		log.Println("Starting MCP server with stdio transport...")
		return server.ServeStdio(s.mcpServer)

	case "http":
		log.Printf("Starting MCP server with HTTP transport on %s%s", s.config.Settings.Server.Address, s.config.Settings.Server.Path)

		if s.stdHTTPServer == nil {
			return fmt.Errorf("HTTP server not initialized")
		}

		// Start HTTP server in a goroutine
		go func() {
			if err := s.stdHTTPServer.ListenAndServe(); err != nil {
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

		// Wait for either context cancellation or server error
		select {
		case <-ctx.Done():
			log.Println("Shutting down server...")
			if err := s.stdHTTPServer.Shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down HTTP server: %v", err)
			}
			return ctx.Err()
		case err := <-errChan:
			log.Printf("Server error received: %v", err)
			return err
		}

	default:
		return fmt.Errorf("unsupported transport: %s", s.config.Settings.Server.Transport)
	}
}

func (s *Server) Close() error {
	if s.stdHTTPServer != nil {
		if err := s.stdHTTPServer.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
		}
	}

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
