package main

import (
	"fmt"
	"log"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/db"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Erro ao carregar config: %v", err)
	}

	conn, err := db.Connect(cfg.Firebird.DSN)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer conn.Close()

	fmt.Println("=== Análise de Estrutura do Banco Firebird ===")

	query := `
		SELECT TRIM(RDB$RELATION_NAME) 
		FROM RDB$RELATIONS 
		WHERE RDB$SYSTEM_FLAG = 0 
		  AND RDB$VIEW_BLR IS NULL
		ORDER BY RDB$RELATION_NAME
	`
	rows, err := conn.Query(query)
	if err != nil {
		log.Fatalf("Erro ao listar tabelas: %v", err)
	}
	defer rows.Close()

	fmt.Println("\nTabelas encontradas (Candidatas à integração):")
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Printf("Erro ao ler nome da tabela: %v", err)
			continue
		}

		// Verifica se já está na tabela de integração
		var ativo string
		err := conn.QueryRow("SELECT ATIVO FROM TABELAS_INTEGRADAS WHERE NOME_TABELA = ?", tableName).Scan(&ativo)
		status := "[ ]"
		if err == nil {
			status = fmt.Sprintf("[%s]", ativo)
		}

		fmt.Printf("%s %s\n", status, tableName)
	}
}
