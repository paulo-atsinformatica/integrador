package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	_ "github.com/nakagami/firebirdsql"
)

// Estrutura para metadados de coluna
type ColumnInfo struct {
	Name string
	Type string
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Uso: go run scripts/install_triggers.go <config_file>")
	}
	configPath := os.Args[1]

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Erro ao carregar config: %v", err)
	}

	db, err := sql.Open("firebirdsql", cfg.Firebird.DSN)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer db.Close()

	// 1. Validar e Criar Tabelas de Suporte se não existirem
	if err := ensureSupportTables(db); err != nil {
		log.Fatalf("Erro ao criar tabelas de suporte: %v", err)
	}

	// 2. Listar tabelas para integrar
	tables, err := getTablesToIntegrate(db)
	if err != nil {
		log.Fatalf("Erro ao listar tabelas: %v", err)
	}

	if len(tables) == 0 {
		log.Println("Nenhuma tabela encontrada em TABELAS_INTEGRADAS com ATIVO='S'.")
		log.Println("Exemplo: INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA) VALUES ('CLIENTES');")
		return
	}

	// 3. Gerar Triggers para cada tabela
	for _, tableName := range tables {
		fmt.Printf("Gerando trigger para %s...\n", tableName)
		if err := createTriggerForTable(db, tableName); err != nil {
			log.Printf("ERRO ao criar trigger para %s: %v", tableName, err)
		} else {
			fmt.Printf("Trigger criada com sucesso para %s.\n", tableName)
		}
	}
}

func ensureSupportTables(db *sql.DB) error {
	// Verifica se a tabela existe
	_, err := db.Exec("SELECT COUNT(*) FROM TABELAS_INTEGRADAS")
	if err != nil {
		// Tabela não existe ou erro, vamos tentar rodar o DDL basico
		// Nota: Idealmente use o setup.sql, aqui é só um fallback simples
		log.Println("Verifique se o script sql/setup.sql foi rodado.")
	}
	return nil
}

func getTablesToIntegrate(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT TRIM(NOME_TABELA) FROM TABELAS_INTEGRADAS WHERE ATIVO = 'S'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func getTableColumns(db *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT TRIM(R.RDB$FIELD_NAME)
		FROM RDB$RELATION_FIELDS R
		WHERE R.RDB$RELATION_NAME = ?
		ORDER BY R.RDB$FIELD_POSITION
	`
	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{Name: name})
	}
	return cols, nil
}

func createTriggerForTable(db *sql.DB, tableName string) error {
	cols, err := getTableColumns(db, tableName)
	if err != nil {
		return err
	}

	// Monta o JSON manual (Concatenação de strings)
	// Ex: '{' || '"CAMPO": "' || COALESCE(NEW.CAMPO, '') || '"' || '}'

	// Função auxiliar para escapar aspas e tratar nulos
	// No FB 2.5 não tem REPLACE nativo fácil sem UDF em alguns casos, mas vamos assumir set simples
	// Para simplificar, vamos concatenar valores simples.
	// ATENÇÃO: Tratamento de strings complexas em FB 2.5 puro é chato. Vamos fazer o básico funcional.

	triggerName := fmt.Sprintf("TRG_SYNC_%s", tableName)
	if len(triggerName) > 31 {
		triggerName = triggerName[:31]
	}

	pkCol := cols[0].Name // Assumindo primeira coluna como PK por simplicidade por enquanto

	// Construção do Payload JSON
	var jsonParts []string
	for i, col := range cols {
		// Formato: "COL": "VALOR"
		// Usamos '' para substituir ' por nada (simplificação) ou convertemos tipos.
		// FB 2.5 Cast: CAST(NEW.COL AS VARCHAR(100))
		part := fmt.Sprintf(" '\"%s\": \"' || COALESCE(CAST(NEW.%s AS VARCHAR(200)), '') || '\"'", col.Name, col.Name)
		jsonParts = append(jsonParts, part)

		if i < len(cols)-1 {
			jsonParts = append(jsonParts, " || ',' || ") // Virgula separadora
		}
	}

	jsonPayload := strings.Join(jsonParts, "")

	// Trigger SQL
	sql := fmt.Sprintf(`
		CREATE OR ALTER TRIGGER %s FOR %s
		ACTIVE AFTER INSERT OR UPDATE OR DELETE POSITION 100
		AS
		DECLARE VARIABLE OP CHAR(1);
		DECLARE VARIABLE PAYLOAD BLOB SUB_TYPE TEXT;
		DECLARE VARIABLE PK_VAL VARCHAR(50);
		BEGIN
			IF (INSERTING) THEN OP = 'I';
			ELSE IF (UPDATING) THEN OP = 'U';
			ELSE OP = 'D';

			-- Monta JSON (Simplificado)
			IF (OP IN ('I', 'U')) THEN
				PAYLOAD = '{' || %s || '}';
			ELSE
				PAYLOAD = NULL; -- Delete envia apenas PK (implementar dps)

			-- Pega valor da PK (Assumindo primeira coluna %s)
			IF (OP = 'D') THEN
				PK_VAL = CAST(OLD.%s AS VARCHAR(50));
			ELSE
				PK_VAL = CAST(NEW.%s AS VARCHAR(50));

			INSERT INTO FILA_INTEGRACAO (EVENT_ID, TABELA, OPERACAO, PK_JSON, PAYLOAD_JSON, ORIGEM)
			VALUES (UUID_TO_CHAR(GEN_UUID()), '%s', :OP, :PK_VAL, :PAYLOAD, 'TRIGGER');
		END
	`, triggerName, tableName, jsonPayload, pkCol, pkCol, pkCol, tableName)

	_, err = db.Exec(sql)
	return err
}
