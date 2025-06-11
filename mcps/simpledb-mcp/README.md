# SimpleDB MCP

A Model Context Protocol (MCP) server for securely accessing and exploring relational databases. Built in Go with cross-platform credential management and biometric authentication.

## Features

- **Database Support**: MySQL and PostgreSQL with connection pooling
- **Secure Credentials**: Cross-platform keychain/credential manager integration
- **Biometric Auth**: TouchID/FaceID on macOS, Windows Hello on Windows
- **Connection Keep-Alive**: Background monitoring keeps database connections healthy
- **Read-Only Operations**: Safe database exploration without modification risks
- **MCP Compatible**: Works with Cursor, Claude CLI, and other MCP clients

## Supported Tools

### Database Exploration
- `list_connections` - Show configured database connections
- `list_databases` - List databases on a connection
- `list_schemas` - List schemas (PostgreSQL only)
- `list_tables` - List tables in a database/schema
- `describe_table` - Show table structure and columns
- `list_indexes` - Show table indexes
- `get_table_sample` - Get sample rows from a table

### Connection Monitoring
- `get_connection_status` - Get connection pool status and health information
- `get_pool_metrics` - Get overall connection pool metrics and statistics

## Installation

### Quick Install (macOS)

```bash
git clone https://github.com/eliziario/simpledb-mcp
cd simpledb-mcp
make install-macos
```

### Quick Install (Windows)

```powershell
git clone https://github.com/eliziario/simpledb-mcp
cd simpledb-mcp
# Run as Administrator
.\scripts\install-windows.ps1
```

### Manual Build

```bash
git clone https://github.com/eliziario/simpledb-mcp
cd simpledb-mcp
make build-local  # or: make build-all for cross-platform
```

### Configuration

The installation scripts automatically set up configuration directories and copy example files. To configure manually:

1. Run the configuration tool:
   ```bash
   simpledb-cli config  # Interactive TUI
   ```

2. Or edit configuration file directly:
   ```bash
   # Configuration is stored in:
   # macOS/Linux: ~/.config/simpledb-mcp/config.yaml
   # Windows: %USERPROFILE%\.config\simpledb-mcp\config.yaml
   ```

## Usage

### As MCP Server

The server runs as an MCP provider using stdio transport:

```bash
./bin/simpledb-mcp
```

### With Cursor

Add to your Cursor MCP configuration:

```json
{
  "mcpServers": {
    "simpledb": {
      "command": "/path/to/simpledb-mcp",
      "args": []
    }
  }
}
```

### With Claude CLI

Register as an MCP provider:

```bash
claude-mcp add simpledb /path/to/simpledb-mcp
```

## Configuration Format

```yaml
connections:
  my-mysql:
    type: mysql
    host: localhost
    port: 3306
    database: myapp
    username: dbuser
  
  my-postgres:
    type: postgres
    host: db.example.com
    port: 5432
    database: analytics
    ssl_mode: require
    username: readonly

settings:
  query_timeout: 30s      # Query timeout
  max_rows: 1000          # Max rows per query
  cache_credentials: 5m   # Credential cache duration
  require_biometric: true # Require biometric auth
  
  # Connection pool settings for keeping database connections alive
  connection_pool:
    enable_keepalive: true      # Enable background connection monitoring
    ping_interval: 30s          # How often to ping connections to keep them alive
    max_idle_time: 15m          # Maximum time a connection can be idle before cleanup
    max_error_count: 3          # Maximum consecutive errors before closing connection
    reconnect_delay: 5s         # Delay before attempting to reconnect after error
```

## Security

- Credentials are stored in OS keychain/credential manager
- Biometric authentication required for credential access
- Read-only operations only - no data modification possible
- Query timeouts and row limits prevent resource abuse

## Development

### Project Structure

```
simpledb-mcp/
├── cmd/
│   ├── simpledb-mcp/      # Main server binary
│   └── simpledb-cli/      # Configuration CLI (planned)
├── internal/
│   ├── config/            # Configuration management
│   ├── credentials/       # Cross-platform credential store
│   ├── database/          # Database connections and queries
│   └── tools/             # MCP tool implementations
└── pkg/api/               # Server API
```

### Adding Database Support

1. Add driver import to `internal/database/database.go`
2. Implement database-specific methods in new file (e.g., `oracle.go`)
3. Add type support in configuration and tools

## License

MIT License - see LICENSE file for details.