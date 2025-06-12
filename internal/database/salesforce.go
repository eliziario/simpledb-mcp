package database

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/simpleforce/simpleforce"
)

// SalesforceClient wraps the simpleforce client
type SalesforceClient struct {
	client *simpleforce.Client
}

// NewSalesforceClient creates a new Salesforce client
func NewSalesforceClient(instanceURL, username, password, securityToken string) (*SalesforceClient, error) {
	// Use the provided Salesforce instance URL
	client := simpleforce.NewClient(instanceURL, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)

	// Login with username, password, and security token
	// For Salesforce, the password + security token is concatenated
	fullPassword := password + securityToken

	if err := client.LoginPassword(username, fullPassword, ""); err != nil {
		return nil, fmt.Errorf("failed to login to Salesforce: %w", err)
	}

	return &SalesforceClient{client: client}, nil
}

// ListDatabasesSalesforce returns dummy database info for Salesforce
func (m *Manager) ListDatabasesSalesforce(connectionName string) ([]string, error) {
	// Salesforce doesn't have databases, return dummy info
	return []string{"salesforce_org"}, nil
}

// ListSchemasSalesforce returns dummy schema info for Salesforce
func (m *Manager) ListSchemasSalesforce(connectionName, database string) ([]string, error) {
	// Salesforce doesn't have schemas, return dummy info
	return []string{"default"}, nil
}

// ListTablesSalesforce lists Salesforce objects (equivalent to tables)
func (m *Manager) ListTablesSalesforce(connectionName string) ([]TableInfo, error) {
	// Get connection config
	conn, exists := m.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	// Get Salesforce credentials
	sfCred, err := m.credManager.GetSalesforce(connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Salesforce credentials: %w", err)
	}

	// Create Salesforce client with instance URL from config
	sfClient, err := NewSalesforceClient(conn.Host, sfCred.Username, sfCred.Password, sfCred.SecurityToken)
	if err != nil {
		return nil, err
	}

	// Use DescribeGlobal to get list of all SObjects
	globalDesc, err := sfClient.client.DescribeGlobal()
	if err != nil {
		return nil, fmt.Errorf("failed to describe global Salesforce objects: %w", err)
	}

	var tables []TableInfo

	// Process sobjects from global describe
	if sobjects, ok := (*globalDesc)["sobjects"].([]interface{}); ok {
		for _, sobjectIface := range sobjects {
			if sobject, ok := sobjectIface.(map[string]interface{}); ok {
				name, _ := sobject["name"].(string)
				custom, _ := sobject["custom"].(bool)
				queryable, _ := sobject["queryable"].(bool)

				// Only include queryable objects
				if !queryable || name == "" {
					continue
				}

				tableType := "STANDARD"
				if custom {
					tableType = "CUSTOM"
				}

				tables = append(tables, TableInfo{
					Name:     name,
					Type:     tableType,
					RowCount: nil, // We'll skip row counts for performance
				})
			}
		}
	}

	return tables, nil
}

// DescribeTableSalesforce describes a Salesforce object (equivalent to table structure)
func (m *Manager) DescribeTableSalesforce(connectionName, objectName string) ([]ColumnInfo, error) {
	// Get connection config
	conn, exists := m.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	// Get Salesforce credentials
	sfCred, err := m.credManager.GetSalesforce(connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Salesforce credentials: %w", err)
	}

	// Create Salesforce client with instance URL from config
	sfClient, err := NewSalesforceClient(conn.Host, sfCred.Username, sfCred.Password, sfCred.SecurityToken)
	if err != nil {
		return nil, err
	}

	// Use REST API to describe the SObject
	// Make a direct REST call to describe the object
	sobjectRestPath := fmt.Sprintf("/services/data/v54.0/sobjects/%s/describe", objectName)
	respBody, err := sfClient.client.ApexREST("GET", sobjectRestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Salesforce object %s: %w", objectName, err)
	}

	// Parse the JSON response
	var sobjectDesc map[string]interface{}
	if err := json.Unmarshal(respBody, &sobjectDesc); err != nil {
		return nil, fmt.Errorf("failed to parse describe response for %s: %w", objectName, err)
	}

	var columns []ColumnInfo

	// Extract fields from the describe response
	if fieldsIface, ok := sobjectDesc["fields"]; ok {
		if fields, ok := fieldsIface.([]interface{}); ok {
			for _, fieldIface := range fields {
				if field, ok := fieldIface.(map[string]interface{}); ok {
					name, _ := field["name"].(string)
					fieldType, _ := field["type"].(string)
					nillable, _ := field["nillable"].(bool)

					// Map Salesforce field types to our column info
					mappedType := mapSalesforceFieldType(fieldType)

					// Get default value if available
					var defaultValue *string
					if defVal, exists := field["defaultValue"]; exists && defVal != nil {
						if defStr, ok := defVal.(string); ok && defStr != "" {
							defaultValue = &defStr
						}
					}

					columns = append(columns, ColumnInfo{
						Name:         name,
						Type:         mappedType,
						Nullable:     nillable,
						DefaultValue: defaultValue,
						IsPrimaryKey: name == "Id", // In Salesforce, Id is always the primary key
					})
				}
			}
		}
	}

	return columns, nil
}

// ListIndexesSalesforce returns dummy index info for Salesforce objects
func (m *Manager) ListIndexesSalesforce(connectionName, objectName string) ([]IndexInfo, error) {
	// Salesforce handles indexing automatically, return basic info
	return []IndexInfo{
		{
			Name:    "PK_" + objectName + "_Id",
			Columns: []string{"Id"},
			Type:    "PRIMARY",
			Unique:  true,
		},
	}, nil
}

// GetTableSampleSalesforce gets sample records from a Salesforce object
func (m *Manager) GetTableSampleSalesforce(connectionName, objectName string, limit int) (map[string]interface{}, error) {
	// Get connection config
	conn, exists := m.config.GetConnection(connectionName)
	if !exists {
		return nil, fmt.Errorf("connection '%s' not found", connectionName)
	}

	// Get Salesforce credentials
	sfCred, err := m.credManager.GetSalesforce(connectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Salesforce credentials: %w", err)
	}

	// Create Salesforce client with instance URL from config
	sfClient, err := NewSalesforceClient(conn.Host, sfCred.Username, sfCred.Password, sfCred.SecurityToken)
	if err != nil {
		return nil, err
	}

	// First, get field names for the object using REST API
	sobjectRestPath := fmt.Sprintf("/services/data/v54.0/sobjects/%s/describe", objectName)
	respBody, err := sfClient.client.ApexREST("GET", sobjectRestPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Salesforce object %s: %w", objectName, err)
	}

	// Parse the JSON response
	var sobjectDesc map[string]interface{}
	if err := json.Unmarshal(respBody, &sobjectDesc); err != nil {
		return nil, fmt.Errorf("failed to parse describe response for %s: %w", objectName, err)
	}

	// Build field list (limit to first 20 fields for performance)
	var fieldNames []string
	maxFields := 20

	if fieldsIface, ok := sobjectDesc["fields"]; ok {
		if fields, ok := fieldsIface.([]interface{}); ok {
			for i, fieldIface := range fields {
				if i >= maxFields {
					break
				}
				if field, ok := fieldIface.(map[string]interface{}); ok {
					fieldName, _ := field["name"].(string)
					fieldType, _ := field["type"].(string)

					// Skip complex field types that might cause issues
					if fieldType == "address" || fieldType == "location" {
						continue
					}
					if fieldName != "" {
						fieldNames = append(fieldNames, fieldName)
					}
				}
			}
		}
	}

	if len(fieldNames) == 0 {
		fieldNames = []string{"Id"} // Fallback to just Id
	}

	// Build SOQL query
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT %d",
		strings.Join(fieldNames, ", "), objectName, limit)

	result, err := sfClient.client.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query Salesforce object %s: %w", objectName, err)
	}

	// Convert results to our format
	var results []map[string]interface{}
	for _, record := range result.Records {
		row := make(map[string]interface{})
		for _, fieldName := range fieldNames {
			val := record[fieldName]
			if val != nil {
				// Clean text fields for JSON safety
				if str, ok := val.(string); ok {
					row[fieldName] = cleanTextForJSON(str)
				} else {
					row[fieldName] = val
				}
			} else {
				row[fieldName] = nil
			}
		}
		results = append(results, row)
	}

	return map[string]interface{}{
		"columns":       fieldNames,
		"rows":          results,
		"total_sampled": len(results),
	}, nil
}

// mapSalesforceFieldType maps Salesforce field types to more standard types
func mapSalesforceFieldType(sfType string) string {
	switch strings.ToLower(sfType) {
	case "id":
		return "varchar(18)" // Salesforce IDs are 15 or 18 characters
	case "string", "textarea", "url", "email", "phone":
		return "text"
	case "picklist", "multipicklist":
		return "varchar(255)"
	case "boolean":
		return "boolean"
	case "int", "integer":
		return "int"
	case "double", "currency", "percent":
		return "decimal"
	case "date":
		return "date"
	case "datetime":
		return "datetime"
	case "reference":
		return "varchar(18)" // References are also 18-character IDs
	case "address":
		return "json" // Complex address type
	case "location":
		return "json" // Complex location type
	default:
		return "text" // Default to text for unknown types
	}
}
