package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/database"
	"github.com/metoro-io/mcp-golang"
)

type Handler struct {
	dbManager *database.Manager
	config    *config.Config
	server    *mcp_golang.Server
}

// Tool argument structures
type ListConnectionsArgs struct{}

type ListDatabasesArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
}

type ListSchemasArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
	Database   string `json:"database" jsonschema:"required,description=Name of the database"`
}

type ListTablesArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
	Database   string `json:"database" jsonschema:"required,description=Name of the database"`
	Schema     string `json:"schema,omitempty" jsonschema:"description=Name of the schema (PostgreSQL only, optional)"`
}

type DescribeTableArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
	Database   string `json:"database" jsonschema:"required,description=Name of the database"`
	Table      string `json:"table" jsonschema:"required,description=Name of the table"`
	Schema     string `json:"schema,omitempty" jsonschema:"description=Name of the schema (PostgreSQL only, optional)"`
}

type ListIndexesArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
	Database   string `json:"database" jsonschema:"required,description=Name of the database"`
	Table      string `json:"table" jsonschema:"required,description=Name of the table"`
	Schema     string `json:"schema,omitempty" jsonschema:"description=Name of the schema (PostgreSQL only, optional)"`
}

type GetTableSampleArgs struct {
	Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
	Database   string `json:"database" jsonschema:"required,description=Name of the database"`
	Table      string `json:"table" jsonschema:"required,description=Name of the table"`
	Schema     string `json:"schema,omitempty" jsonschema:"description=Name of the schema (PostgreSQL only, optional)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"minimum=1,maximum=100,description=Number of rows to sample (default: 10, max: 100)"`
}

type GetConnectionStatusArgs struct {
	Connection string `json:"connection,omitempty" jsonschema:"description=Name of specific connection (optional - if not provided, returns all)"`
}

type GetPoolMetricsArgs struct{}

// Helper function to create JSON content
func newJSONContent(data interface{}) (*mcp_golang.Content, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return mcp_golang.NewTextContent(string(jsonData)), nil
}

func NewHandler(dbManager *database.Manager, config *config.Config, server *mcp_golang.Server) (*Handler, error) {
	h := &Handler{
		dbManager: dbManager,
		config:    config,
		server:    server,
	}
	if err := h.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}
	return h, nil
}

func (h *Handler) registerTools() error {
	// Register all tools
	if err := h.server.RegisterTool("list_connections", "List all configured database connections", h.listConnections); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("list_databases", "List databases available on a connection", h.listDatabases); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("list_schemas", "List schemas in a database (PostgreSQL only)", h.listSchemas); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("list_tables", "List tables in a database/schema", h.listTables); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("describe_table", "Show table structure including columns, types, and constraints", h.describeTable); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("list_indexes", "Show indexes for a table", h.listIndexes); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("get_table_sample", "Get a sample of rows from a table", h.getTableSample); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("get_connection_status", "Get connection pool status and health information", h.getConnectionStatus); err != nil {
		return err
	}
	
	if err := h.server.RegisterTool("get_pool_metrics", "Get overall connection pool metrics and statistics", h.getPoolMetrics); err != nil {
		return err
	}
	
	return nil
}

func (h *Handler) listConnections(args ListConnectionsArgs) (*mcp_golang.ToolResponse, error) {
	var connections []map[string]interface{}
	
	for name, conn := range h.config.Connections {
		status := "unknown"
		if err := h.dbManager.TestConnection(name); err == nil {
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

	content, err := newJSONContent(connections)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) listDatabases(args ListDatabasesArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

	var databases []string
	var err error

   switch conn.Type {
   case "mysql":
       databases, err = h.dbManager.ListDatabasesMySQL(args.Connection)
   case "postgres":
       databases, err = h.dbManager.ListDatabasesPostgres(args.Connection)
   case "salesforce":
       databases, err = h.dbManager.ListDatabasesSalesforce(args.Connection)
   case "glue":
       databases, err = h.dbManager.ListDatabasesGlue(args.Connection)
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }

	if err != nil {
		return nil, err
	}

	content, err := newJSONContent(databases)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) listSchemas(args ListSchemasArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

   switch conn.Type {
   case "postgres":
       schemas, err := h.dbManager.ListSchemasPostgres(args.Connection, args.Database)
       if err != nil {
           return nil, err
       }
       content, err := newJSONContent(schemas)
       if err != nil {
           return nil, err
       }
       return mcp_golang.NewToolResponse(content), nil
   case "mysql":
       return nil, fmt.Errorf("MySQL does not support schemas - use list_databases instead")
   case "salesforce":
       schemas, err := h.dbManager.ListSchemasSalesforce(args.Connection, args.Database)
       if err != nil {
           return nil, err
       }
       content, err := newJSONContent(schemas)
       if err != nil {
           return nil, err
       }
       return mcp_golang.NewToolResponse(content), nil
   case "glue":
       schemas, err := h.dbManager.ListSchemasGlue(args.Connection, args.Database)
       if err != nil {
           return nil, err
       }
       content, err := newJSONContent(schemas)
       if err != nil {
           return nil, err
       }
       return mcp_golang.NewToolResponse(content), nil
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }
}

func (h *Handler) listTables(args ListTablesArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

	var tables []database.TableInfo
	var err error

   switch conn.Type {
   case "mysql":
       tables, err = h.dbManager.ListTablesMySQL(args.Connection, args.Database)
   case "postgres":
       tables, err = h.dbManager.ListTablesPostgres(args.Connection, args.Database, args.Schema)
   case "salesforce":
       tables, err = h.dbManager.ListTablesSalesforce(args.Connection)
   case "glue":
       tables, err = h.dbManager.ListTablesGlue(args.Connection, args.Database, args.Schema)
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }

	if err != nil {
		return nil, err
	}

	content, err := newJSONContent(tables)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) describeTable(args DescribeTableArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

	var columns []database.ColumnInfo
	var err error

   switch conn.Type {
   case "mysql":
       columns, err = h.dbManager.DescribeTableMySQL(args.Connection, args.Database, args.Table)
   case "postgres":
       columns, err = h.dbManager.DescribeTablePostgres(args.Connection, args.Database, args.Table, args.Schema)
   case "salesforce":
       columns, err = h.dbManager.DescribeTableSalesforce(args.Connection, args.Table)
   case "glue":
       columns, err = h.dbManager.DescribeTableGlue(args.Connection, args.Database, args.Table, args.Schema)
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }

	if err != nil {
		return nil, err
	}

	content, err := newJSONContent(columns)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) listIndexes(args ListIndexesArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

	var indexes []database.IndexInfo
	var err error

   switch conn.Type {
   case "mysql":
       indexes, err = h.dbManager.ListIndexesMySQL(args.Connection, args.Database, args.Table)
   case "postgres":
       indexes, err = h.dbManager.ListIndexesPostgres(args.Connection, args.Database, args.Table, args.Schema)
   case "salesforce":
       indexes, err = h.dbManager.ListIndexesSalesforce(args.Connection, args.Table)
   case "glue":
       indexes, err = h.dbManager.ListIndexesGlue(args.Connection, args.Database, args.Table)
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }

	if err != nil {
		return nil, err
	}

	content, err := newJSONContent(indexes)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) getTableSample(args GetTableSampleArgs) (*mcp_golang.ToolResponse, error) {
	conn, exists := h.config.GetConnection(args.Connection)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", args.Connection)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 3 // default - keep low for readability
	}

	// Enforce max limit
	if limit > int(h.config.Settings.MaxRows) {
		limit = int(h.config.Settings.MaxRows)
	}
	if limit > 100 {
		limit = 100
	}

	var sample map[string]interface{}
	var err error

   switch conn.Type {
   case "mysql":
       sample, err = h.dbManager.GetTableSampleMySQL(args.Connection, args.Database, args.Table, limit)
   case "postgres":
       sample, err = h.dbManager.GetTableSamplePostgres(args.Connection, args.Database, args.Table, args.Schema, limit)
   case "salesforce":
       sample, err = h.dbManager.GetTableSampleSalesforce(args.Connection, args.Table, limit)
   case "glue":
       sample, err = h.dbManager.GetTableSampleGlue(args.Connection, args.Database, args.Table, limit)
   default:
       return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
   }

	if err != nil {
		return nil, err
	}

	// Return array of dictionaries for better readability
	rows, ok := sample["rows"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid sample data format: expected rows array")
	}
	
	content, err := newJSONContent(rows)
	if err != nil {
		return nil, err
	}
	return mcp_golang.NewToolResponse(content), nil
}

func (h *Handler) getConnectionStatus(args GetConnectionStatusArgs) (*mcp_golang.ToolResponse, error) {
	if args.Connection != "" {
		// Get status for specific connection
		status := h.dbManager.GetConnectionStatus(args.Connection)
		return mcp_golang.NewToolResponse(
			mcp_golang.NewTextContent(fmt.Sprintf("Connection '%s': %s (last used: %s, idle: %s)", 
				status.Name, status.State, status.LastUsed.Format("15:04:05"), status.IdleTime.Truncate(time.Second))),
		), nil
	} else {
		// Get status for all connections
		statuses := h.dbManager.GetAllConnectionStatus()
		if len(statuses) == 0 {
			return mcp_golang.NewToolResponse(
				mcp_golang.NewTextContent("No active connections in pool"),
			), nil
		}
		
		statusText := fmt.Sprintf("Connection pool status (%d connections):\n", len(statuses))
		for _, status := range statuses {
			statusText += fmt.Sprintf("  • %s: %s (idle: %s, errors: %d)\n", 
				status.Name, status.State, status.IdleTime.Truncate(time.Second), status.ErrorCount)
		}
		
		return mcp_golang.NewToolResponse(
			mcp_golang.NewTextContent(statusText),
		), nil
	}
}

func (h *Handler) getPoolMetrics(args GetPoolMetricsArgs) (*mcp_golang.ToolResponse, error) {
	metrics := h.dbManager.GetPoolMetrics()
	
	metricsText := fmt.Sprintf(`Connection Pool Metrics:
  Total connections created: %d
  Active connections: %d
  Currently connected: %d
  Connections with errors: %d
  Successful pings: %d
  Failed pings: %d
  Ping interval: %s
  Max idle time: %s
  Success rate: %.1f%%`,
		metrics.TotalConnections,
		metrics.ActiveConnections,
		metrics.ConnectedCount,
		metrics.ErrorCount,
		metrics.SuccessfulPings,
		metrics.FailedPings,
		metrics.PingInterval,
		metrics.MaxIdleTime,
		float64(metrics.SuccessfulPings) / float64(metrics.SuccessfulPings + metrics.FailedPings) * 100,
	)
	
	return mcp_golang.NewToolResponse(
		mcp_golang.NewTextContent(metricsText),
	), nil
}