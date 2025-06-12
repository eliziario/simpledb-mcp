# Release Notes

## v0.3.0 - AWS Glue Integration

### 🚀 New Features

**AWS Glue Data Catalog Support**
- Full integration with AWS Glue Data Catalog for metadata access
- Support for listing databases, tables, and schema information
- Athena integration for actual data sampling from tables
- Compatible with existing AWS infrastructure and security policies

**Flexible MFA Authentication**
- **Native macOS Dialog** (default): Clean, user-friendly dialog for manual MFA code entry
- **Automated gauth Integration**: Power-user option using existing gauth tools
- Configurable per-connection via `use_gauth` parameter
- Follows established aws_mfa script authentication patterns

**Enterprise-Grade Security**
- STS assume-role with MFA authentication
- Auto-refreshing credentials (refreshes 1 minute before expiry)
- Secure credential caching and session management
- Integration with existing IAM roles and policies

### 🔧 Technical Implementation

**AWS Integration Architecture**
- Uses AWS SDK Go with proper STS credential provider
- Background credential refresh with thread-safe caching
- Athena query execution with configurable timeouts
- S3 integration for Athena result storage

**Configuration System**
- Extended connection config to support AWS-specific parameters
- Support for role ARN, MFA serial, and authentication preferences
- Environment variable integration for Athena S3 output location

**MCP Tool Compatibility**
- All existing MCP tools work seamlessly with Glue connections
- Proper error handling and timeout management
- JSON response formatting consistent with other database types

### 📚 Documentation

- Comprehensive AWS Glue setup guide in README
- Clear configuration examples for both MFA authentication methods
- Required IAM permissions documentation
- Environment variable setup instructions

### 🛠️ Developer Experience

- Clean separation between authentication methods
- Platform-specific implementations (macOS-optimized dialogs)
- Proper error messages and validation
- Consistent with existing codebase patterns

---

## v0.2.0 - Salesforce Integration

### 🚀 New Features

**Salesforce Support**
- Added comprehensive Salesforce integration alongside MySQL and PostgreSQL
- Full MCP tool support for Salesforce objects:
  - `list_tables` - Lists all queryable Salesforce objects (standard and custom)
  - `describe_table` - Shows object fields, types, and metadata  
  - `get_table_sample` - Retrieves sample records using SOQL queries
  - `list_databases`/`list_schemas` - Return placeholder values for MCP client compatibility

**Salesforce Authentication**
- Secure credential storage in OS keychain with biometric authentication
- Support for username, password, and security token authentication
- Configurable Salesforce instance URLs (sandbox, production, custom domains)

**Enhanced Configuration**
- Added `host` field support for Salesforce connection configuration
- Updated example configuration with Salesforce examples
- New credential storage utility: `store_sf_creds.go`

### 🔧 Technical Improvements

**Salesforce Implementation**
- Uses `simpleforce` Go library for SOQL queries and REST API access
- Intelligent field filtering (limits to 20 most relevant fields for performance)
- Salesforce-to-SQL type mapping for consistent MCP responses
- Graceful handling of complex field types (address, location)
- Automatic text field sanitization for JSON safety

**Code Architecture**
- Extended credential management system to support multiple credential types
- Added platform-specific biometric authentication for Salesforce credentials
- Modular database abstraction supporting multiple backend types
- Enhanced error handling and logging

### 📚 Documentation

- Updated README with comprehensive Salesforce setup guide
- Added Salesforce-specific configuration examples
- Enhanced security documentation
- Updated example configuration files

### 🛠️ Developer Experience

- New utility script for easy Salesforce credential storage
- Enhanced build process with proper dependency management
- Improved error messages and debugging information

### 🔒 Security

- All Salesforce operations are read-only (SELECT/DESCRIBE only)
- Credentials encrypted in OS keychain with biometric protection
- Query timeouts and row limits prevent resource abuse
- No direct SOQL execution - only predefined safe operations

---

## v0.1.0 - Initial Release

### 🚀 Features

**Database Support**
- MySQL and PostgreSQL connection support
- Cross-platform credential management with biometric authentication
- Connection pooling with keep-alive monitoring

**MCP Integration**
- Complete Model Context Protocol server implementation
- Comprehensive database exploration tools
- HTTP and stdio transport support

**Security**
- TouchID/FaceID on macOS, Windows Hello on Windows  
- Read-only database operations
- Secure credential storage in OS keychain

**Tools**
- Database exploration: list databases, schemas, tables
- Table analysis: describe structure, list indexes
- Data sampling: get sample rows with customizable limits
- Connection monitoring: health checks and pool metrics