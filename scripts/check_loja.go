package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/nakagami/firebirdsql"
)

func main() {
	dsn := "SYSDBA:masterkey@localhost:3050/E:/reswincs/banco/TESTE.fb"
	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, _ := db.Query("SELECT FIRST 5 EVENT_ID, TABELA, STATUS, PK_JSON, PAYLOAD_JSON FROM FILA_INTEGRACAO ORDER BY ID DESC")
	if rows != nil {
		for rows.Next() {
			var eid, tab, st, pk, py string
			rows.Scan(&eid, &tab, &st, &pk, &py)
			fmt.Printf("ID: %s | Tab: %6s | Stat: %s | PK: %s | Data: %s\n", eid, tab, st, pk, py)
		}
		rows.Close()
	}

	fmt.Println("\n--- CLIENTE (LOJA) ---")
	var nome string
	err = db.QueryRow("SELECT NOMEFANTASIA FROM CLIENTE WHERE CODCLIENTE = '00000100'").Scan(&nome)
	if err != nil {
		fmt.Printf("Erro cliente: %v\n", err)
	} else {
		fmt.Printf("NOMEFANTASIA: %s\n", nome)
	}

	fmt.Println("\n--- PRODUTO (LOJA) ---")
	var desc string
	err = db.QueryRow("SELECT DESCRICAO FROM PRODUTO WHERE CODPROD = '999'").Scan(&desc)
	if err != nil {
		fmt.Printf("Erro produto: %v\n", err)
	} else {
		fmt.Printf("DESCRICAO: %s\n", desc)
	}
}
