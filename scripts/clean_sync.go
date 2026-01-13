package main

import (
	"database/sql"
	"log"

	_ "github.com/nakagami/firebirdsql"
)

func clean(dsn string) {
	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		log.Printf("Erro ao abrir %s: %v", dsn, err)
		return
	}
	defer db.Close()
	_, _ = db.Exec("DELETE FROM FILA_DESTINOS")
	_, _ = db.Exec("DELETE FROM SYNC_NODES")
	_, _ = db.Exec("DELETE FROM FILA_INTEGRACAO")
	log.Println("Limpou " + dsn)
}

func main() {
	clean("SYSDBA:masterkey@localhost:3050/E:/reswincs/banco/TESTE.fb")
	clean("SYSDBA:masterkey@localhost:3050/E:/reswincs/banco/TESTE2.fb")
}
