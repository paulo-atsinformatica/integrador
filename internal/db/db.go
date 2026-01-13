package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/nakagami/firebirdsql"
)

// Connect abre uma conexão com o Firebird
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no Firebird: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erro de ping no Firebird: %w", err)
	}

	return db, nil
}

// GetPKColumns retorna os nomes das colunas que compõem a PK de uma tabela
func GetPKColumns(db *sql.DB, tableName string) ([]string, error) {
	query := `
		SELECT 
			iseg.rdb$field_name
		FROM rdb$indices idx
		JOIN rdb$index_segments iseg ON idx.rdb$index_name = iseg.rdb$index_name
		JOIN rdb$relation_constraints rc ON idx.rdb$index_name = rc.rdb$index_name
		WHERE rc.rdb$relation_name = ? AND rc.rdb$constraint_type = 'PRIMARY KEY'
		ORDER BY iseg.rdb$field_position
	`
	rows, err := db.Query(query, strings.ToUpper(tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkCols []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		pkCols = append(pkCols, strings.TrimSpace(colName))
	}
	return pkCols, nil
}
