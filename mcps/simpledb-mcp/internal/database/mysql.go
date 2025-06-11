package database

import (
	"database/sql"
	"fmt"
)

func (m *Manager) ListDatabasesMySQL(connectionName string) ([]string, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SHOW DATABASES")
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

func (m *Manager) ListTablesMySQL(connectionName, database string) ([]TableInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_ROWS, 0) as TABLE_ROWS
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = ? 
		ORDER BY TABLE_NAME`

	rows, err := db.Query(query, database)
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
		if rowCount.Valid {
			table.RowCount = &rowCount.Int64
		}
		tables = append(tables, table)
	}

	return tables, nil
}

func (m *Manager) DescribeTableMySQL(connectionName, database, tableName string) ([]ColumnInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			COLUMN_NAME,
			COLUMN_TYPE,
			IS_NULLABLE = 'YES' as IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_KEY = 'PRI' as IS_PRIMARY_KEY
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

	rows, err := db.Query(query, database, tableName)
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

func (m *Manager) ListIndexesMySQL(connectionName, database, tableName string) ([]IndexInfo, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			INDEX_NAME,
			COLUMN_NAME,
			INDEX_TYPE,
			NON_UNIQUE = 0 as IS_UNIQUE
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.Query(query, database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var indexName, columnName, indexType string
		var isUnique bool
		if err := rows.Scan(&indexName, &columnName, &indexType, &isUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index info: %w", err)
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &IndexInfo{
				Name:    indexName,
				Columns: []string{columnName},
				Type:    indexType,
				Unique:  isUnique,
			}
		}
	}

	var indexes []IndexInfo
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

func (m *Manager) GetTableSampleMySQL(connectionName, database, tableName string, limit int) (map[string]interface{}, error) {
	db, err := m.GetConnection(connectionName)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT * FROM `%s`.`%s` LIMIT %d", database, tableName, limit)
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
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
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