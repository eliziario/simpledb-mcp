#!/usr/bin/env python3
"""
Test script for SimpleDB MCP Server tools
Tests all available MCP tools via HTTP transport
Uses only Python 3.13 standard library
"""

import json
import sys
import urllib.request
import urllib.parse
import urllib.error
from typing import Dict, Any, List
from dataclasses import dataclass


@dataclass
class TestResult:
    tool_name: str
    success: bool
    response: Dict[str, Any]
    error: str = ""


class MCPTester:
    def __init__(self, base_url: str = "http://localhost:8080/mcp"):
        self.base_url = base_url
        
    def send_request(self, method: str, params: Dict[str, Any] = None) -> Dict[str, Any]:
        """Send a JSON-RPC request to the MCP server"""
        request_id = 1
        payload = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": params or {}
        }
        
        try:
            # Prepare the request
            data = json.dumps(payload).encode('utf-8')
            req = urllib.request.Request(
                self.base_url,
                data=data,
                headers={
                    'Content-Type': 'application/json',
                    'Content-Length': str(len(data))
                }
            )
            
            # Send the request
            with urllib.request.urlopen(req, timeout=30) as response:
                response_data = response.read().decode('utf-8')
                return json.loads(response_data)
                
        except urllib.error.HTTPError as e:
            return {"error": f"HTTP Error {e.code}: {e.reason}"}
        except urllib.error.URLError as e:
            return {"error": f"URL Error: {e.reason}"}
        except json.JSONDecodeError as e:
            return {"error": f"JSON Decode Error: {str(e)}"}
        except Exception as e:
            return {"error": f"Request failed: {str(e)}"}
    
    def initialize_server(self) -> bool:
        """Initialize the MCP server"""
        print("🔧 Initializing MCP server...")
        response = self.send_request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "test-client", "version": "1.0.0"}
        })
        
        if "error" in response:
            print(f"❌ Failed to initialize: {response['error']}")
            return False
            
        if "result" in response:
            print("✅ Server initialized successfully")
            return True
            
        print(f"❌ Unexpected response: {response}")
        return False
    
    def get_available_tools(self) -> List[Dict[str, Any]]:
        """Get list of available tools from the server"""
        print("📋 Getting available tools...")
        response = self.send_request("tools/list")
        
        if "error" in response:
            print(f"❌ Failed to get tools: {response['error']}")
            return []
            
        if "result" in response and "tools" in response["result"]:
            tools = response["result"]["tools"]
            print(f"✅ Found {len(tools)} tools")
            return tools
            
        print(f"❌ Unexpected response: {response}")
        return []
    
    def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> TestResult:
        """Call a specific tool with given arguments"""
        response = self.send_request("tools/call", {
            "name": tool_name,
            "arguments": arguments
        })
        
        if "error" in response:
            return TestResult(
                tool_name=tool_name,
                success=False,
                response=response,
                error=response["error"]
            )
        
        if "result" in response:
            result = response["result"]
            is_error = result.get("isError", False)
            return TestResult(
                tool_name=tool_name,
                success=not is_error,
                response=response,
                error=result.get("content", [{}])[0].get("text", "") if is_error else ""
            )
        
        return TestResult(
            tool_name=tool_name,
            success=False,
            response=response,
            error="Unexpected response format"
        )
    
    def test_list_connections(self) -> TestResult:
        """Test the list_connections tool"""
        return self.call_tool("list_connections", {})
    
    def test_get_pool_metrics(self) -> TestResult:
        """Test the get_pool_metrics tool"""
        return self.call_tool("get_pool_metrics", {})
    
    def test_get_connection_status(self) -> TestResult:
        """Test the get_connection_status tool"""
        # Test with no specific connection (should return all)
        return self.call_tool("get_connection_status", {})
    
    def test_with_connection(self, connection_name: str) -> List[TestResult]:
        """Test tools that require a connection name"""
        results = []
        
        # Test list_databases
        results.append(self.call_tool("list_databases", {
            "connection": connection_name
        }))
        
        # Test get_connection_status for specific connection
        results.append(self.call_tool("get_connection_status", {
            "connection": connection_name
        }))
        
        return results
    
    def test_with_database(self, connection_name: str, database_name: str) -> List[TestResult]:
        """Test tools that require connection and database"""
        results = []
        
        # Test list_tables
        results.append(self.call_tool("list_tables", {
            "connection": connection_name,
            "database": database_name
        }))
        
        # Test list_schemas (PostgreSQL only, but won't hurt to try)
        results.append(self.call_tool("list_schemas", {
            "connection": connection_name,
            "database": database_name
        }))
        
        return results
    
    def test_with_table(self, connection_name: str, database_name: str, table_name: str) -> List[TestResult]:
        """Test tools that require connection, database, and table"""
        results = []
        
        # Test describe_table

        results.append(self.call_tool("describe_table", {
            "connection": connection_name,
            "database": database_name,
            "table": table_name
        }))
        
        # Test list_indexes
        results.append(self.call_tool("list_indexes", {
            "connection": connection_name,
            "database": database_name,
            "table": table_name
        }))
        
        # Test get_table_sample
        results.append(self.call_tool("get_table_sample", {
            "connection": connection_name,
            "database": database_name,
            "table": table_name,
            "limit": 5
        }))
        
        return results
    
    def extract_databases_from_response(self, result: TestResult) -> List[str]:
        """Extract database names from list_databases response"""
        if not result.success or result.error:
            return []
        
        try:
            content = result.response["result"]["content"]
            databases = []
            
            # Look for database names in the response content
            for item in content:
                if item.get("type") == "text":
                    text = item.get("text", "")
                    
                    # Try to parse as JSON array first (new format)
                    try:
                        import json
                        databases = json.loads(text)
                        if isinstance(databases, list):
                            break
                    except (json.JSONDecodeError, ValueError):
                        pass
                    
                    # Handle old format like "[db1 db2 db3 db4]" - space separated in brackets
                    if text.startswith("[") and text.endswith("]"):
                        # Remove brackets and split by spaces
                        db_string = text[1:-1].strip()
                        if db_string:
                            databases = db_string.split()
                    # Handle format: "Found databases: db1, db2, db3"
                    elif "databases:" in text.lower():
                        parts = text.split(":")
                        if len(parts) > 1:
                            db_list = parts[1].strip().split(",")
                            databases.extend([db.strip() for db in db_list if db.strip()])
            
            # Filter out common system databases and limit results
            filtered_databases = []
            system_dbs = {"information_schema", "performance_schema", "mysql", "sys", "innodb"}
            
            for db in databases:
                if db.lower() not in system_dbs:
                    filtered_databases.append(db)
                if len(filtered_databases) >= 3:  # Max 3 user databases
                    break
            
            # If no user databases found, include a few system ones for testing
            if not filtered_databases:
                for db in databases:
                    if db.lower() in {"information_schema", "mysql"}:
                        filtered_databases.append(db)
                    if len(filtered_databases) >= 2:
                        break
            
            return filtered_databases
        except (KeyError, IndexError, TypeError):
            return []
    
    def extract_tables_from_response(self, result: TestResult) -> List[str]:
        """Extract table names from list_tables response"""
        if not result.success or result.error:
            return []
        
        try:
            content = result.response["result"]["content"]
            tables = []
            
            # Look for table names in the response content
            for item in content:
                if item.get("type") == "text":
                    text = item.get("text", "")
                    
                    # Try to parse as JSON array first (new format)
                    try:
                        import json
                        table_data = json.loads(text)
                        if isinstance(table_data, list):
                            # Extract table names from TableInfo objects or strings
                            for table in table_data:
                                if isinstance(table, dict) and "name" in table:
                                    tables.append(table["name"])
                                elif isinstance(table, str):
                                    tables.append(table)
                            break
                    except (json.JSONDecodeError, ValueError):
                        pass
                    
                    # Handle old format like "[table1 table2 table3]" - space separated in brackets
                    if text.startswith("[") and text.endswith("]"):
                        # Remove brackets and split by spaces
                        table_string = text[1:-1].strip()
                        if table_string:
                            tables = table_string.split()
                    # Handle format: "Found tables: table1, table2, table3"
                    elif "tables:" in text.lower():
                        parts = text.split(":")
                        if len(parts) > 1:
                            table_list = parts[1].strip().split(",")
                            tables.extend([t.strip() for t in table_list if t.strip()])
            
            return tables[:2]  # Return max 2 tables to avoid too many tests
        except (KeyError, IndexError, TypeError):
            return []

    def extract_connections_from_response(self, result: TestResult) -> List[Dict[str, Any]]:
        """Extract connection info from list_connections response"""
        if not result.success or result.error:
            return []
        
        try:
            content = result.response["result"]["content"]
            for item in content:
                if item.get("type") == "text":
                    text = item.get("text", "")
                    try:
                        import json
                        connections = json.loads(text)
                        if isinstance(connections, list):
                            return connections
                    except (json.JSONDecodeError, ValueError):
                        pass
            return []
        except (KeyError, IndexError, TypeError):
            return []

    def get_database_type(self, connections: List[Dict[str, Any]], connection_name: str) -> str:
        """Get database type for a connection"""
        for conn in connections:
            if conn.get("name") == connection_name:
                return conn.get("type", "unknown")
        return "unknown"

    def run_comprehensive_test(self) -> None:
        """Run comprehensive test of all tools with smart discovery"""
        print("🚀 Starting comprehensive MCP tool test\n")
        
        # Initialize server
        if not self.initialize_server():
            sys.exit(1)
        
        # Get available tools
        tools = self.get_available_tools()
        if not tools:
            sys.exit(1)
        
        print("\n📝 Available tools:")
        for tool in tools:
            print(f"  • {tool['name']}: {tool['description']}")
        
        all_results = []
        
        print("\n🧪 Step 1: Testing basic tools...")
        
        # Test basic tools that don't require parameters
        basic_tests = [
            self.test_list_connections(),
            self.test_get_pool_metrics(),
            self.test_get_connection_status()
        ]
        all_results.extend(basic_tests)
        
        # Extract connection information
        connections_result = basic_tests[0]
        connections = self.extract_connections_from_response(connections_result)
        
        if not connections:
            print("❌ No connections found, cannot proceed with database tests")
            self.print_test_summary(all_results)
            return
        
        print(f"✅ Found {len(connections)} connections")
        
        # Test each connection
        for connection_info in connections:
            connection_name = connection_info["name"]
            db_type = connection_info["type"]
            
            print(f"\n🔗 Step 2: Testing connection '{connection_name}' (type: {db_type})")
            
            # Test connection-specific tools
            connection_tests = self.test_with_connection(connection_name)
            all_results.extend(connection_tests)
            
            # Get databases
            databases_result = connection_tests[0]  # list_databases result
            if not databases_result.success or databases_result.error:
                print(f"❌ Failed to list databases for {connection_name}")
                continue
                
            databases = self.extract_databases_from_response(databases_result)
            if not databases:
                print(f"⚠️  No databases found for {connection_name}")
                continue
                
            print(f"✅ Found databases: {databases[:3]}{'...' if len(databases) > 3 else ''}")
            
            # Test with first available database
            test_database = databases[0]
            print(f"\n📊 Step 3: Testing database operations on '{test_database}'")
            
            # Test schemas if PostgreSQL
            if db_type == "postgres":
                print(f"  🏗️  Testing schemas (PostgreSQL)")
                schemas_result = self.call_tool("list_schemas", {
                    "connection": connection_name,
                    "database": test_database
                })
                all_results.append(schemas_result)
                
                if schemas_result.success:
                    schemas = self.extract_databases_from_response(schemas_result)  # Reuse same logic
                    if schemas:
                        print(f"  ✅ Found schemas: {schemas}")
                        test_schema = schemas[0]
                    else:
                        test_schema = "public"  # Default PostgreSQL schema
                else:
                    test_schema = "public"
            else:
                test_schema = None
            
            # Test list_tables
            print(f"  📋 Testing tables in database '{test_database}'")
            tables_args = {
                "connection": connection_name,
                "database": test_database
            }
            if test_schema:
                tables_args["schema"] = test_schema
                
            tables_result = self.call_tool("list_tables", tables_args)
            all_results.append(tables_result)
            
            if not tables_result.success or tables_result.error:
                print(f"  ❌ Failed to list tables")
                continue
                
            tables = self.extract_tables_from_response(tables_result)
            if not tables:
                print(f"  ⚠️  No tables found")
                continue
                
            print(f"  ✅ Found tables: {tables[:5]}{'...' if len(tables) > 5 else ''}")
            
            # Test with first available table
            test_table = tables[0]
            print(f"\n🗂️  Step 4: Testing table operations on '{test_table}'")
            
            # Prepare table arguments
            table_args = {
                "connection": connection_name,
                "database": test_database,
                "table": test_table
            }
            if test_schema:
                table_args["schema"] = test_schema
            
            # Test describe_table
            print(f"    🔍 Describing table structure")
            describe_result = self.call_tool("describe_table", table_args)
            all_results.append(describe_result)
            
            # Test list_indexes
            print(f"    📇 Listing table indexes")
            indexes_result = self.call_tool("list_indexes", table_args)
            all_results.append(indexes_result)
            
            # Test get_table_sample
            print(f"    📄 Getting table sample (5 rows)")
            sample_args = table_args.copy()
            sample_args["limit"] = 5
            sample_result = self.call_tool("get_table_sample", sample_args)
            all_results.append(sample_result)
            
            if sample_result.success:
                print(f"    ✅ Retrieved sample data")
            else:
                print(f"    ❌ Failed to get sample: {sample_result.error}")
            
            # Only test first connection to avoid too many requests
            break
        
        # Print summary
        print(f"\n🎯 Testing completed!")
        self.print_test_summary(all_results)
    
    def print_test_summary(self, results: List[TestResult]) -> None:
        """Print a summary of all test results"""
        print("\n" + "="*60)
        print("📊 TEST SUMMARY")
        print("="*60)
        
        success_count = sum(1 for r in results if r.success)
        total_count = len(results)
        
        print(f"\nOverall: {success_count}/{total_count} tests passed")
        
        # Group by success/failure
        successful = [r for r in results if r.success]
        failed = [r for r in results if not r.success]
        
        if successful:
            print(f"\n✅ Successful tests ({len(successful)}):")
            for result in successful:
                print(f"  • {result.tool_name}")
        
        if failed:
            print(f"\n❌ Failed tests ({len(failed)}):")
            for result in failed:
                print(f"  • {result.tool_name}: {result.error}")
        
        print("\n" + "="*60)


def main():
    """Main function"""
    import argparse
    
    parser = argparse.ArgumentParser(description="Test SimpleDB MCP Server tools")
    parser.add_argument(
        "--url", 
        default="http://localhost:8080/mcp",
        help="MCP server URL (default: http://localhost:8080/mcp)"
    )
    parser.add_argument(
        "--tool",
        help="Test specific tool only"
    )
    
    args = parser.parse_args()
    
    tester = MCPTester(args.url)
    
    if args.tool:
        # Test specific tool
        print(f"🧪 Testing tool: {args.tool}")
        if not tester.initialize_server():
            sys.exit(1)
        
        result = tester.call_tool(args.tool, {})
        if result.success:
            print(f"✅ {args.tool} - Success")
            print(json.dumps(result.response, indent=2))
        else:
            print(f"❌ {args.tool} - Failed: {result.error}")
            sys.exit(1)
    else:
        # Run comprehensive test
        tester.run_comprehensive_test()


if __name__ == "__main__":
    main()