package database

import (
   "fmt"
   "os"
   "time"

   "github.com/aws/aws-sdk-go/aws"
   "github.com/aws/aws-sdk-go/service/athena"
   "github.com/aws/aws-sdk-go/service/glue"
)


// ListDatabasesGlue lists all Glue Catalog databases.
func (m *Manager) ListDatabasesGlue(connectionName string) ([]string, error) {
   sess, err := m.glueSession(connectionName)
   if err != nil {
       return nil, err
   }
   svc := glue.New(sess)
   input := &glue.GetDatabasesInput{}
   var outNames []string
   for {
       resp, err := svc.GetDatabases(input)
       if err != nil {
           return nil, err
       }
       for _, db := range resp.DatabaseList {
           outNames = append(outNames, aws.StringValue(db.Name))
       }
       if resp.NextToken == nil {
           break
       }
       input.NextToken = resp.NextToken
   }
   return outNames, nil
}

// ListSchemasGlue returns the database name as the only schema.
func (m *Manager) ListSchemasGlue(connectionName, database string) ([]string, error) {
   return []string{database}, nil
}

// ListTablesGlue lists tables in a Glue database.
func (m *Manager) ListTablesGlue(connectionName, database, _ string) ([]TableInfo, error) {
   sess, err := m.glueSession(connectionName)
   if err != nil {
       return nil, err
   }
   svc := glue.New(sess)
   input := &glue.GetTablesInput{DatabaseName: aws.String(database)}
   var tables []TableInfo
   for {
       resp, err := svc.GetTables(input)
       if err != nil {
           return nil, err
       }
       for _, t := range resp.TableList {
           tables = append(tables, TableInfo{
               Name:     aws.StringValue(t.Name),
               Type:     aws.StringValue(t.TableType),
               RowCount: nil,
           })
       }
       if resp.NextToken == nil {
           break
       }
       input.NextToken = resp.NextToken
   }
   return tables, nil
}

// DescribeTableGlue retrieves column definitions for a Glue table.
func (m *Manager) DescribeTableGlue(connectionName, database, tableName, _ string) ([]ColumnInfo, error) {
   sess, err := m.glueSession(connectionName)
   if err != nil {
       return nil, err
   }
   svc := glue.New(sess)
   resp, err := svc.GetTable(&glue.GetTableInput{
       DatabaseName: aws.String(database),
       Name:         aws.String(tableName),
   })
   if err != nil {
       return nil, err
   }
   sd := resp.Table.StorageDescriptor
   var cols []ColumnInfo
   for _, c := range sd.Columns {
       cols = append(cols, ColumnInfo{
           Name:         aws.StringValue(c.Name),
           Type:         aws.StringValue(c.Type),
           Nullable:     true,
           DefaultValue: nil,
           IsPrimaryKey: false,
       })
   }
   return cols, nil
}

// ListIndexesGlue always returns nil since Glue has no indexes.
func (m *Manager) ListIndexesGlue(connectionName, database, tableName string) ([]IndexInfo, error) {
   return nil, nil
}

// GetTableSampleGlue runs an Athena query to sample rows.
func (m *Manager) GetTableSampleGlue(connectionName, database, tableName string, limit int) (map[string]interface{}, error) {
   sess, err := m.glueSession(connectionName)
   if err != nil {
       return nil, err
   }
   
   // Get Athena S3 output location from config, fallback to environment variable
   conn, exists := m.config.GetConnection(connectionName)
   if !exists {
       return nil, fmt.Errorf("connection %s not found", connectionName)
   }
   
   outLoc := conn.AthenaS3Output
   if outLoc == "" {
       outLoc = os.Getenv("AWS_ATHENA_S3_OUTPUT")
   }
   if outLoc == "" {
       return nil, fmt.Errorf("athena_s3_output must be set in connection config or AWS_ATHENA_S3_OUTPUT environment variable for Athena results")
   }
   
   ath := athena.New(sess)
   query := fmt.Sprintf("SELECT * FROM \"%s\".\"%s\" LIMIT %d", database, tableName, limit)
   si, err := ath.StartQueryExecution(&athena.StartQueryExecutionInput{
       QueryString: aws.String(query),
       QueryExecutionContext: &athena.QueryExecutionContext{Database: aws.String(database)},
       ResultConfiguration:  &athena.ResultConfiguration{OutputLocation: aws.String(outLoc)},
   })
   if err != nil {
       return nil, err
   }
   qid := aws.StringValue(si.QueryExecutionId)
   deadline := time.Now().Add(m.config.Settings.QueryTimeout)
   for {
       ge, err := ath.GetQueryExecution(&athena.GetQueryExecutionInput{QueryExecutionId: aws.String(qid)})
       if err != nil {
           return nil, err
       }
       st := aws.StringValue(ge.QueryExecution.Status.State)
       if st == "SUCCEEDED" {
           break
       }
       if st == "FAILED" || st == "CANCELLED" {
           return nil, fmt.Errorf("Athena query %s: %s", st, aws.StringValue(ge.QueryExecution.Status.StateChangeReason))
       }
       if time.Now().After(deadline) {
           return nil, fmt.Errorf("Athena query timed out after %s", m.config.Settings.QueryTimeout)
       }
       time.Sleep(time.Second)
   }
   gr, err := ath.GetQueryResults(&athena.GetQueryResultsInput{QueryExecutionId: aws.String(qid)})
   if err != nil {
       return nil, err
   }
   rows := gr.ResultSet.Rows
   if len(rows) < 1 {
       return map[string]interface{}{"columns": []string{}, "rows": []map[string]interface{}{}, "total_sampled": 0}, nil
   }
   header := rows[0].Data
   var cols []string
   for _, d := range header {
       cols = append(cols, aws.StringValue(d.VarCharValue))
   }
   var outRows []map[string]interface{}
   for _, r := range rows[1:] {
       m := make(map[string]interface{}, len(cols))
       for i, d := range r.Data {
           if i < len(cols) {
               m[cols[i]] = aws.StringValue(d.VarCharValue)
           }
       }
       outRows = append(outRows, m)
   }
   return map[string]interface{}{
       "columns":       cols,
       "rows":          outRows,
       "total_sampled": len(outRows),
   }, nil
}