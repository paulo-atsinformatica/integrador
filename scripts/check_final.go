package main

import (
	"database/sql"
	"fmt"

	_ "github.com/nakagami/firebirdsql"
)

func check(label, dsn string) {
	db, _ := sql.Open("firebirdsql", dsn)
	defer db.Close()

	fmt.Printf("--- %s (%s) ---\n", label, dsn)
	fmt.Println("SYNC_NODES:")
	rows, _ := db.Query("SELECT NODE_ID, REMOTE_URL, ACTIVE FROM SYNC_NODES")
	for rows.Next() {
		var id, url, act string
		rows.Scan(&id, &url, &act)
		fmt.Printf("  %s | %s | active:%s\n", id, url, act)
	}

	fmt.Println("FILA_INTEGRACAO (Last 2):")
	rows, _ = db.Query("SELECT FIRST 2 ID, EVENT_ID, TABELA, STATUS, ORIGEM FROM FILA_INTEGRACAO ORDER BY ID DESC")
	for rows.Next() {
		var id int64
		var eid, tab, st, ori string
		rows.Scan(&id, &eid, &tab, &st, &ori)
		fmt.Printf("  ID:%d | %s | %s | Stat:%s | Ori:%s\n", id, eid, tab, st, ori)

		fmt.Println("  FILA_DESTINOS:")
		rowsD, _ := db.Query("SELECT NODE_ID, STATUS, ERRO_MSG FROM FILA_DESTINOS WHERE FILA_ID = ?", id)
		for rowsD.Next() {
			var nid, dst, err string
			rowsD.Scan(&nid, &dst, &err)
			fmt.Printf("    -> %s | Stat:%s | Err:%v\n", nid, dst, err)
		}
	}
}

func main() {
	check("LOJA", "SYSDBA:masterkey@localhost:3050/E:/reswincs/banco/TESTE.fb")
	check("CENTRAL", "SYSDBA:masterkey@localhost:3050/E:/reswincs/banco/TESTE2.fb")
}
