package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/trace"
)

type DataResolver struct {
	db    *sql.DB
	queue *QueueManager
}

func NewDataResolver(db *sql.DB, queue *QueueManager) *DataResolver {
	return &DataResolver{db: db, queue: queue}
}

// Resolve toma uma lista de eventos de uma transação commitada e os processa
func (r *DataResolver) Resolve(events []*trace.TraceEvent) error {
	for _, event := range events {
		// 1. Verifica se a tabela deve ser integrada
		if !r.isTableIntegrated(event.Table) {
			continue
		}

		// 2. Identifica as colunas de PK
		pkCols, err := GetPKColumns(r.db, event.Table)
		if err != nil || len(pkCols) == 0 {
			fmt.Printf("Aviso: Tabela %s sem PK definida ou erro na busca. Pulando.\n", event.Table)
			continue
		}

		// 3. Extrai valores de PK do SQL (Aproximação)
		// No FB 2.5, extrair valores do SQL do Trace é complexo.
		// Para fins deste agente, vamos assumir que extraímos os valores da PK
		// ou que o Trace nos deu contexto suficiente.
		pkValues := r.extractPKValuesFromSQL(event.SQL, pkCols)

		if event.Type == trace.EventDelete {
			// Para Delete, enviamos apenas a PK
			r.queue.Insert(event.Table, "D", pkValues, nil)
			continue
		}

		// 4. Faz snapshot para Insert/Update
		snapshot, err := r.fetchSnapshot(event.Table, pkCols, pkValues)
		if err != nil {
			fmt.Printf("Erro ao buscar snapshot de %s: %v\n", event.Table, err)
			continue
		}

		r.queue.Insert(event.Table, string(event.Type)[0:1], pkValues, snapshot)
	}
	return nil
}

func (r *DataResolver) isTableIntegrated(table string) bool {
	var ativo string
	err := r.db.QueryRow("SELECT ATIVO FROM TABELAS_INTEGRADAS WHERE NOME_TABELA = ?", strings.ToUpper(table)).Scan(&ativo)
	return err == nil && ativo == "S"
}

func (r *DataResolver) fetchSnapshot(table string, pkCols []string, pkValues map[string]interface{}) (map[string]interface{}, error) {
	whereClauses := []string{}
	args := []interface{}{}
	for _, col := range pkCols {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", col))
		args = append(args, pkValues[col])
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s", table, strings.Join(whereClauses, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("registro não encontrado")
	}

	cols, _ := rows.Columns()
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, col := range cols {
		val := values[i]
		if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = val
		}
	}

	return result, nil
}

func (r *DataResolver) extractPKValuesFromSQL(sqlText string, pkCols []string) map[string]interface{} {
	// Implementação simplificada: em um cenário real, precisaríamos de um parser de SQL
	// ou capturar os parâmetros do Trace se estivessem ativos.
	// Por enquanto, retornamos um mapa vazio que precisará ser populado via lógica de parsing do SQL.
	res := make(map[string]interface{})
	// ... (Lógica de parser de SQL viria aqui)
	return res
}
