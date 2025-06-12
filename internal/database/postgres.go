package database

import (
	"database/sql"
	"fmt"
)

func (m *Manager) ListDatabasesPostgres(connectionName string) ([]string, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT datname 
		FROM pg_database 
		WHERE datistemplate = false 
		ORDER BY datname`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, dbName)
	}

	return databases, nil
}

func (m *Manager) ListSchemasPostgres(connectionName, database string) ([]string, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT schema_name 
		FROM information_schema.schemata 
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY schema_name`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemas = append(schemas, schemaName)
	}

	return schemas, nil
}

func (m *Manager) ListTablesPostgres(connectionName, database, schema string) ([]TableInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	if schema == "" {
		schema = "public"
	}

	query := `
		SELECT 
			t.table_name,
			t.table_type,
			COALESCE(c.reltuples::bigint, 0) as estimated_rows
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.nspname = t.table_schema AND c.relnamespace = n.oid
		WHERE t.table_schema = $1
		ORDER BY t.table_name`

	rows, err := db.Query(query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		var rowCount sql.NullInt64
		if err := rows.Scan(&table.Name, &table.Type, &rowCount); err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}
		if rowCount.Valid && rowCount.Int64 > 0 {
			table.RowCount = &rowCount.Int64
		}
		tables = append(tables, table)
	}

	return tables, nil
}

func (m *Manager) DescribeTablePostgres(connectionName, database, tableName, schema string) ([]ColumnInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	if schema == "" {
		schema = "public"
	}

	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES' as is_nullable,
			c.column_default,
			COALESCE(tc.constraint_type = 'PRIMARY KEY', false) as is_primary_key
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu 
			ON c.table_name = kcu.table_name 
			AND c.column_name = kcu.column_name
			AND c.table_schema = kcu.table_schema
		LEFT JOIN information_schema.table_constraints tc
			ON kcu.constraint_name = tc.constraint_name
			AND tc.constraint_type = 'PRIMARY KEY'
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`

	rows, err := db.Query(query, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var defaultValue sql.NullString
		if err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultValue, &col.IsPrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		columns = append(columns, col)
	}

	return columns, nil
}

func (m *Manager) ListIndexesPostgres(connectionName, database, tableName, schema string) ([]IndexInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	if schema == "" {
		schema = "public"
	}

	query := `
		SELECT 
			i.indexname,
			i.indexdef,
			ix.indisunique
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.tablename
		JOIN pg_namespace n ON n.nspname = i.schemaname AND c.relnamespace = n.oid
		JOIN pg_index ix ON ix.indexrelid = (
			SELECT oid FROM pg_class WHERE relname = i.indexname AND relnamespace = n.oid
		)
		WHERE i.schemaname = $1 AND i.tablename = $2
		ORDER BY i.indexname`

	rows, err := db.Query(query, schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var indexName, indexDef string
		var isUnique bool
		if err := rows.Scan(&indexName, &indexDef, &isUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index info: %w", err)
		}

		// Parse column names from index definition (simplified)
		// In a production system, you'd want more robust parsing
		indexes = append(indexes, IndexInfo{
			Name:    indexName,
			Columns: []string{}, // TODO: Parse from indexdef
			Type:    "btree",    // Default for Postgres
			Unique:  isUnique,
		})
	}

	return indexes, nil
}

func (m *Manager) GetTableSamplePostgres(connectionName, database, tableName, schema string, limit int) (map[string]interface{}, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`SELECT * FROM "%s"."%s" LIMIT %d`, schema, tableName, limit)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table sample: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
			} else if b, ok := val.([]byte); ok {
				// Handle byte arrays (TEXT, VARCHAR, etc.)
				text := string(b)
				// Escape and clean text for JSON safety
				row[col] = cleanTextForJSON(text)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	return map[string]interface{}{
		"columns":       columns,
		"rows":          results,
		"total_sampled": len(results),
	}, nil
}
