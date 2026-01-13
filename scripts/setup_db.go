package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/db"
)

func main() {
	fmt.Println("=== Iniciando Setup do Banco de Dados Firebird ===")

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Erro ao carregar config: %v", err)
	}

	conn, err := db.Connect(cfg.Firebird.DSN)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer conn.Close()

	statements := []string{
		`CREATE TABLE TABELAS_INTEGRADAS (
			NOME_TABELA VARCHAR(31) NOT NULL PRIMARY KEY,
			ATIVO CHAR(1) DEFAULT 'S' CHECK (ATIVO IN ('S', 'N'))
		)`,
		`CREATE TABLE FILA_INTEGRACAO (
			ID DOUBLE PRECISION NOT NULL PRIMARY KEY,
			EVENT_ID CHAR(36) NOT NULL,
			TABELA VARCHAR(31) NOT NULL,
			OPERACAO CHAR(1) NOT NULL,
			PK_JSON BLOB SUB_TYPE 1,
			PAYLOAD_JSON BLOB SUB_TYPE 1,
			ORIGEM VARCHAR(20),
			STATUS CHAR(1) DEFAULT 'P',
			TENTATIVAS INTEGER DEFAULT 0,
			DT_EVENTO TIMESTAMP DEFAULT 'NOW',
			DT_ULT_ENVIO TIMESTAMP,
			ERRO_MSG BLOB SUB_TYPE 1
		)`,
		`CREATE GENERATOR GEN_FILA_INTEGRACAO_ID`,
		`CREATE TRIGGER TRG_FILA_INTEGRACAO_BI FOR FILA_INTEGRACAO
		ACTIVE BEFORE INSERT POSITION 0
		AS
		BEGIN
			IF (NEW.ID IS NULL) THEN
				NEW.ID = NEXT VALUE FOR GEN_FILA_INTEGRACAO_ID;
		END`,
		`CREATE INDEX IDX_FILA_STATUS ON FILA_INTEGRACAO (STATUS)`,
		`CREATE UNIQUE INDEX IDX_FILA_EVENT_ID ON FILA_INTEGRACAO (EVENT_ID)`,
		// Inserts iniciais
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('CLIENTE', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('PRODUTO', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('CADOPER', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('FORNECE', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('UNIDADES', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('PEDIDOC', 'S')`,
		`INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES ('PEDIDOI', 'S')`,
	}

	for _, stmt := range statements {
		fmt.Printf("Executando: %s...\n", strings.Fields(stmt)[1])
		_, err := conn.Exec(stmt)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "defined more than once") {
				fmt.Println("  [OK] Já existe.")
				continue
			}
			log.Printf("  [ERRO] %v\n", err)
		} else {
			fmt.Println("  [OK] Criado com sucesso.")
		}
	}

	fmt.Println("\n=== Setup Concluído com Sucesso! ===")
}
