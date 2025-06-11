# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SimpleDB MCP is a Model Context Protocol server that provides secure, read-only access to MySQL and PostgreSQL databases. It features cross-platform credential management with biometric authentication (TouchID/FaceID on macOS, Windows Hello on Windows).

## Common Development Commands

### Building
```bash
# Build main server
go build -o bin/simpledb-mcp ./cmd/simpledb-mcp

# Build with cross-compilation
GOOS=darwin GOARCH=amd64 go build -o bin/simpledb-mcp-darwin ./cmd/simpledb-mcp
GOOS=windows GOARCH=amd64 go build -o bin/simpledb-mcp-windows.exe ./cmd/simpledb-mcp
```

### Testing
```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/config
go test ./internal/database

# Test with verbose output
go test -v ./...
```

### Dependencies
```bash
# Update dependencies
go mod tidy

# Add new dependency
go get github.com/example/package
```

## Architecture

### Core Components

- **MCP Server** (`pkg/api/server.go`): Main server using metoro-io/mcp-golang with stdio transport
- **Configuration** (`internal/config/`): YAML-based config in `~/.config/simpledb-mcp/`
- **Credentials** (`internal/credentials/`): Cross-platform keychain integration with biometric auth
- **Database** (`internal/database/`): Connection pooling with keep-alive and database-specific query implementations
- **Connection Pool** (`internal/database/pool.go`): Background monitoring, health checks, and connection lifecycle management
- **Tools** (`internal/tools/`): MCP tool definitions with type-safe argument structures

### Build Tags and Platform Support

The project uses Go build tags for platform-specific biometric authentication:
- `biometric_darwin.go`: TouchID/FaceID support using `go-touchid`
- `biometric_windows.go`: Windows Hello placeholder (uses regular credential manager)
- `biometric_other.go`: Fallback for Linux and other systems

### Database Abstraction

Database operations are abstracted through the `Manager` type with separate implementation files:
- `mysql.go`: MySQL-specific queries using INFORMATION_SCHEMA
- `postgres.go`: PostgreSQL-specific queries using information_schema and pg_catalog

### MCP Tool Pattern

Tools are implemented using struct-based argument definitions with JSON schema tags:
```go
type DescribeTableArgs struct {
    Connection string `json:"connection" jsonschema:"required,description=Name of the database connection"`
    Database   string `json:"database" jsonschema:"required,description=Name of the database"`
    Table      string `json:"table" jsonschema:"required,description=Name of the table"`
    Schema     string `json:"schema,omitempty" jsonschema:"description=Name of the schema (PostgreSQL only, optional)"`
}
```

## Key Dependencies

- `github.com/metoro-io/mcp-golang`: MCP protocol implementation
- `github.com/zalando/go-keyring`: Cross-platform credential storage
- `github.com/ansxuman/go-touchid`: macOS biometric authentication
- `github.com/go-sql-driver/mysql`: MySQL driver
- `github.com/lib/pq`: PostgreSQL driver
- `gopkg.in/yaml.v3`: Configuration parsing

## Security Considerations

- All database operations are read-only (SELECT, SHOW, DESCRIBE only)
- Credentials stored in OS keychain/credential manager
- Biometric authentication required for credential access
- Query timeouts and row limits prevent resource abuse
- No direct SQL query execution - only predefined tool functions

## Configuration

Default config location: `~/.config/simpledb-mcp/config.yaml`
Example config in: `configs/example-config.yaml`

Connections require credentials to be stored separately in keychain using future CLI tool.

### Connection Pool Features

The connection pool provides:
- **Background Keep-Alive**: Automatic ping monitoring to keep connections healthy
- **Error Recovery**: Automatic reconnection on failures with configurable retry limits
- **Idle Cleanup**: Removes unused connections after configurable idle time
- **Health Monitoring**: Real-time connection status and metrics via MCP tools
- **Graceful Shutdown**: Proper cleanup of all connections on server stop

Key settings in config:
- `enable_keepalive`: Enable/disable background monitoring
- `ping_interval`: How often to ping idle connections
- `max_idle_time`: When to cleanup unused connections
- `max_error_count`: Consecutive errors before closing connection

## Future CLI Tool

Plan includes `cmd/simpledb-cli/` with TUI interface for:
- Connection management
- Credential storage with biometric auth
- Service installation
- Configuration validation