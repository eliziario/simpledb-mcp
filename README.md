# SimpleDB MCP

A Model Context Protocol (MCP) server for securely accessing and exploring relational databases. Built in Go with cross-platform credential management and biometric authentication.

## Features

- **Database Support**: MySQL, PostgreSQL, Salesforce, and AWS Glue with connection pooling
- **Secure Credentials**: Cross-platform keychain/credential manager integration
- **Biometric Auth**: TouchID/FaceID on macOS, Windows Hello on Windows
- **Connection Keep-Alive**: Background monitoring keeps database connections healthy
- **Read-Only Operations**: Safe database exploration without modification risks
- **MCP Compatible**: Works with Claude Desktop, Cursor, Claude CLI, and other MCP clients

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

### With Claude Desktop

SimpleDB MCP includes a stdio-to-HTTP proxy for Claude Desktop compatibility while keeping the advantages of a long-running HTTP server.

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "simpledb": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:48384/mcp"]
    }
  }
}
```

The proxy automatically forwards stdio requests from Claude Desktop to your running SimpleDB MCP HTTP server. See [Claude Integration Guide](docs/claude-integration.md) for detailed setup instructions.

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
  
  my-salesforce:
    type: salesforce
    host: https://mycompany.my.salesforce.com
    # Credentials stored separately in keychain
  
  my-glue:
    type: glue
    host: us-east-1  # AWS region
    role_arn: arn:aws:iam::123456789012:role/AdminRole
    mfa_serial: arn:aws:iam::123456789012:mfa/your.username
    athena_s3_output: s3://your-athena-results-bucket/results/

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

## Salesforce Integration

SimpleDB MCP provides secure access to Salesforce objects through SOQL queries:

### Salesforce Setup

1. **Store Salesforce credentials**:
   ```bash
   go run store_sf_creds.go my-salesforce https://mycompany.my.salesforce.com user@company.com password security_token
   ```

2. **Get your Security Token**:
   - Log into Salesforce → Settings → My Personal Information → Reset My Security Token
   - The token will be emailed to you

3. **Salesforce Tools**:
   - `list_tables` - Lists all queryable Salesforce objects (standard and custom)
   - `describe_table` - Shows object fields, types, and metadata
   - `get_table_sample` - Retrieves sample records using SOQL
   - `list_databases`/`list_schemas` - Return placeholder values for MCP client compatibility

### Salesforce Features

- **Read-Only Access**: Uses SOQL SELECT queries only
- **Field Filtering**: Automatically limits to 20 most relevant fields for performance
- **Type Mapping**: Converts Salesforce field types to standard SQL equivalents
- **Error Handling**: Graceful handling of complex field types (address, location)

## AWS Glue Integration

SimpleDB MCP provides secure access to AWS Glue Data Catalog and Athena for table sampling:

### AWS Glue Setup

1. **Configure AWS MFA authentication** (choose one option):
   
   **Option A: Native macOS Dialog (Recommended)**
   - Set `use_gauth: false` or omit this field in your connection config
   - Enter MFA codes manually from your authenticator app via native dialog
   
   **Option B: Automated with gauth**
   - Set `use_gauth: true` in your connection config
   - Ensure you have the aws_mfa script configured in `~/.config/.aws_menu.ini`
   - Install and configure gauth for automated MFA token generation
   
   - Your IAM user must have permission to assume the specified role

2. **Required AWS Permissions**:
   - `glue:GetDatabases`, `glue:GetTables`, `glue:GetTable` for catalog access
   - `athena:StartQueryExecution`, `athena:GetQueryExecution`, `athena:GetQueryResults` for sampling
   - `s3:GetBucketLocation`, `s3:GetObject`, `s3:ListBucket`, `s3:PutObject` for Athena results

3. **Athena S3 Output Configuration**:
   Set the S3 location for Athena query results in your connection config:
   ```yaml
   my-glue:
     type: glue
     athena_s3_output: s3://your-athena-results-bucket/results/
   ```
   
   Alternatively, you can use the environment variable:
   ```bash
   export AWS_ATHENA_S3_OUTPUT="s3://your-athena-results-bucket/results/"
   ```

4. **AWS Glue Tools**:
   - `list_databases` - Lists all Glue Catalog databases
   - `list_tables` - Lists tables in a Glue database  
   - `describe_table` - Shows table schema from Glue Catalog
   - `get_table_sample` - Executes Athena queries to sample table data
   - `list_schemas` - Returns database name (Glue uses database-level organization)

### AWS Glue Features

- **Flexible MFA Authentication**: 
  - Native macOS dialog for manual MFA code entry (default)
  - Automated gauth integration for power users
- **Auto-refresh**: STS credentials automatically refresh when expired
- **Athena Integration**: Table sampling uses Athena for actual data queries
- **Pagination**: Handles large numbers of databases/tables efficiently
- **Timeout Protection**: Configurable query timeouts prevent long-running queries

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