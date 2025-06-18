# Claude Desktop Integration

This guide explains how to configure Claude Desktop and Claude Code to work with SimpleDB MCP using the stdio-to-HTTP proxy.

## Overview

SimpleDB MCP runs as a long-running HTTP server (via launchd on macOS or Windows Services), which provides several advantages like connection pooling, background monitoring, and persistent authentication. However, Claude Desktop and Claude Code only communicate with MCP servers via stdio.

To bridge this gap, SimpleDB MCP includes a **stdio-to-HTTP proxy** (`simpledb-mcp-proxy`) that:
- Accepts stdio input from Claude clients
- Forwards JSON-RPC requests to the HTTP server
- Returns responses back via stdio

## Architecture

```
┌─────────────────┐    stdio    ┌─────────────────┐    HTTP    ┌─────────────────┐
│  Claude Desktop │────────────▶│ simpledb-mcp-   │───────────▶│ simpledb-mcp    │
│  / Claude Code  │◀────────────│ proxy           │◀───────────│ HTTP Server     │
└─────────────────┘             └─────────────────┘            └─────────────────┘
                                                                        │
                                                                        ▼
                                                                ┌─────────────────┐
                                                                │ Database        │
                                                                │ Connections     │
                                                                └─────────────────┘
```

## Prerequisites

1. **Install SimpleDB MCP** using the installation scripts:
   - macOS: `make install-macos`
   - Windows: Run `scripts/install-windows.ps1` as Administrator

2. **Ensure the HTTP server is running**:
   - macOS: `launchctl list | grep simpledb`
   - Windows: `Get-Service -Name SimpleDBMCP`

3. **Configure your database connections** in `~/.config/simpledb-mcp/config.yaml`

## Claude Desktop Configuration

### 1. Locate Claude Desktop Configuration

The Claude Desktop configuration file is located at:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

### 2. Add SimpleDB MCP Configuration

Add the following configuration to your `claude_desktop_config.json`:

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

If you already have other MCP servers configured, add the `simpledb` entry to your existing `mcpServers` object.

### 3. Complete Configuration Example

```json
{
  "mcpServers": {
    "simpledb": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:48384/mcp"]
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/files"]
    }
  }
}
```

### 4. Custom Server Configuration

If your SimpleDB MCP HTTP server runs on a different address or port, update the `--server` argument:

```json
{
  "mcpServers": {
    "simpledb": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:9090/mcp"]
    }
  }
}
```

## Claude Code Configuration

Claude Code automatically detects MCP servers in your project's `mcp.json` file. Create a file named `mcp.json` in your project root:

```json

i halist 
```

## Verification

### 1. Test the Proxy

Test the proxy manually to ensure it can communicate with the HTTP server:

```bash
# Start the proxy (it will wait for JSON-RPC input)
simpledb-mcp-proxy --server http://localhost:48384/mcp

# In another terminal, test with a simple request:
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | simpledb-mcp-proxy --server http://localhost:48384/mcp
```

### 2. Restart Claude Desktop

After updating the configuration:
1. Completely quit Claude Desktop
2. Restart Claude Desktop
3. The SimpleDB MCP tools should now be available

### 3. Verify in Claude

You should now be able to use commands like:
- "List my database connections"
- "Show me the tables in the [database] database"
- "Describe the structure of the [table] table"
- "Get a sample of data from [table]"

## Troubleshooting

### Common Issues

1. **Proxy not found**: Ensure SimpleDB MCP is properly installed and `simpledb-mcp-proxy` is in your PATH
2. **Connection refused**: Verify the HTTP server is running and accessible
3. **Permission errors**: Check that Claude Desktop has necessary permissions to execute the proxy

### Debug Mode

For debugging, you can run the proxy with additional logging:

```bash
# The proxy logs to stderr, so you can redirect to see debug info
simpledb-mcp-proxy --server http://localhost:48384/mcp 2>/tmp/proxy-debug.log
```

Check the log file for connection issues or other errors.

### Server Status Check

Verify the HTTP server is responding:

```bash
curl -X POST http://localhost:48384/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

### Check Service Status

- **macOS**: `launchctl list | grep simpledb`
- **Windows**: `Get-Service -Name SimpleDBMCP`

If the service isn't running, restart it:
- **macOS**: `launchctl start com.simpledb-mcp.server`
- **Windows**: `Start-Service -Name SimpleDBMCP`

## Advanced Configuration

### Custom Port Configuration

If you need to run the SimpleDB MCP server on a different port, update your `~/.config/simpledb-mcp/config.yaml`:

```yaml
settings:
  server:
    transport: http
    address: :9090
    path: /mcp
```

Then update the Claude configuration to match:

```json
{
  "mcpServers": {
    "simpledb": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:9090/mcp"]
    }
  }
}
```

### Multiple Instances

You can run multiple SimpleDB MCP instances on different ports for different environments:

```json
{
  "mcpServers": {
    "simpledb-prod": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:48384/mcp"]
    },
    "simpledb-dev": {
      "command": "simpledb-mcp-proxy",
      "args": ["--server", "http://localhost:8081/mcp"]
    }
  }
}
```

## Security Considerations

- The proxy only forwards requests to the configured HTTP server
- All security features of the main SimpleDB MCP server apply (biometric authentication, read-only access, etc.)
- The proxy itself doesn't store or cache any credentials
- Communication between proxy and server is over HTTP (consider HTTPS for production)