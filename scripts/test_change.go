package main

import (
	"fmt"
	"log"
	"time"

	"github.com/atsinformatica/firebird-sync-agent/internal/db"
)

func main() {
	// DSN TCP explícita para forçar logs no Trace
	dsn := "SYSDBA:masterkey@localhost:3050/E:/Reswincs/Banco/TESTE.FB?charset=WIN1252"

	fmt.Printf("Conectando via TCP em: %s\n", dsn)
	conn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("Erro ao conectar: %v", err)
	}
	defer conn.Close()

	fmt.Println("Executando UPDATE na tabela CLIENTE...")

	// Pega um cliente qualquer
	var codCliente int
	err = conn.QueryRow("SELECT FIRST 1 CODCLIENTE FROM CLIENTE").Scan(&codCliente)
	if err != nil {
		log.Fatalf("Erro ao buscar cliente: %v", err)
	}

	// Faz um update dummy para gerar log
	tx, err := conn.Begin()
	if err != nil {
		log.Fatalf("Erro ao iniciar transação: %v", err)
	}

	_, err = tx.Exec("UPDATE CLIENTE SET NOMEFANTASIA = SUBSTRING(NOMEFANTASIA FROM 1 FOR 30) || '.' WHERE CODCLIENTE = ?", codCliente)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Erro ao executar update: %v", err)
	}

	fmt.Println("Commitando transação...")
	if err := tx.Commit(); err != nil {
		log.Fatalf("Erro ao commitar: %v", err)
	}

	fmt.Println("Sucesso! Verifique se apareceu [TRACE RAW] no console do agente.")
	time.Sleep(2 * time.Second)
}
