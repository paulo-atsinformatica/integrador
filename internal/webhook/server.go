package webhook

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/db"
	"github.com/atsinformatica/firebird-sync-agent/internal/models"
)

type Server struct {
	dbConn *sql.DB
	queue  *db.QueueManager
	token  string
}

func NewServer(dbConn *sql.DB, queue *db.QueueManager, token string) *Server {
	return &Server{
		dbConn: dbConn,
		queue:  queue,
		token:  token,
	}
}

func (s *Server) Listen(addr string) error {
	http.HandleFunc("/sync", s.handleSync)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Valida Token
	if r.Header.Get("X-Sync-Token") != s.token {
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	var payload models.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}
	payload.RemoteAddr = r.RemoteAddr

	if err := s.ProcessPayload(payload); err != nil {
		log.Printf("[SERVER] Erro ao processar payload: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) ProcessPayload(payload models.SyncPayload) error {
	// 1.5 Auto-Registro de Nós (Multi-Cliente)
	s.registerNode(payload)

	// 2. Verifica Idempotência (Anti-Loop duplicado)
	duplicate, err := s.queue.IsDuplicate(payload.EventID)
	if err != nil {
		return fmt.Errorf("erro interno de banco: %w", err)
	}
	if duplicate {
		log.Printf("[SERVER] Evento duplicado ignorado: %s", payload.EventID)
		return nil
	}

	// 3. Inicia Transação para garantir commit real
	tx, err := s.dbConn.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback() // Se falhar, desfaz

	// 4. Aplica dado no Firebird
	if err := s.applyToDBTx(tx, payload); err != nil {
		return fmt.Errorf("erro ao aplicar no banco remoto: %w", err)
	}

	// 5. Registra na fila local como 'A' (Aplicado) para histórico
	pkJSON, _ := json.Marshal(payload.PKJSON)
	payloadJSON, _ := json.Marshal(payload.PayloadJSON)

	queryQueue := `
		INSERT INTO FILA_INTEGRACAO (EVENT_ID, TABELA, OPERACAO, PK_JSON, PAYLOAD_JSON, ORIGEM, STATUS, TENTATIVAS, DT_EVENTO)
		VALUES (?, ?, ?, ?, ?, ?, 'A', 0, CURRENT_TIMESTAMP)
	`
	if _, err := tx.Exec(queryQueue, payload.EventID, payload.Table, payload.Operation, string(pkJSON), string(payloadJSON), payload.Origem); err != nil {
		log.Printf("[SERVER] Erro ao gravar histórico: %v", err)
	}

	// 6. COMMIT REAL
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao comitar alteração: %w", err)
	}

	return nil
}

func (s *Server) applyToDBTx(tx *sql.Tx, p models.SyncPayload) error {
	if p.Operation == "D" {
		params := []interface{}{}
		clauses := []string{}
		for k, v := range p.PKJSON {
			clauses = append(clauses, fmt.Sprintf("%s = ?", k))
			params = append(params, v)
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s", p.Table, strings.Join(clauses, " AND "))
		_, err := tx.Exec(query, params...)
		return err
	}

	// Merge: PK e Data precisam estar juntos no INSERT/VALUES
	allData := make(map[string]interface{})
	for k, v := range p.PKJSON {
		allData[k] = v
	}
	for k, v := range p.PayloadJSON {
		allData[k] = v
	}

	cols := []string{}
	placeholders := []string{}
	vals := []interface{}{}
	pkCols := []string{}

	if len(allData) == 0 {
		return fmt.Errorf("payload vazio para update/insert")
	}

	for k, v := range allData {
		cols = append(cols, k)
		placeholders = append(placeholders, "?")

		// Especial para Firebird 2.5: Tratar string vazia como NULL para campos numéricos/data
		if s, ok := v.(string); ok && s == "" {
			vals = append(vals, nil)
		} else {
			vals = append(vals, v)
		}
	}

	for k := range p.PKJSON {
		pkCols = append(pkCols, k)
	}

	query := fmt.Sprintf(
		"UPDATE OR INSERT INTO %s (%s) VALUES (%s) MATCHING (%s)",
		p.Table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(pkCols, ", "),
	)

	log.Printf("[SERVER] Aplicando SQL: %s | Valores: %v", query, vals)
	_, err := tx.Exec(query, vals...)
	return err
}

func (s *Server) registerNode(p models.SyncPayload) {
	if p.Origem == "" || p.Origem == "TRIGGER" {
		return
	}

	// Tenta inferir a URL de retorno baseada no IP de quem enviou (assumindo porta 8080 padrão se não especificado)
	// Mas o ideal seria o cliente enviar sua URL de escuta no payload futuramente.
	// Por enquanto, registramos o nó para habilitar o broadcast.
	remoteIP := p.RemoteAddr
	if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
		remoteIP = remoteIP[:idx]
	}
	// TODO: No futuro, o Payload deve trazer a URL de escuta real do cliente (configurada nele)
	remoteURL := fmt.Sprintf("http://%s:8080/sync", remoteIP)

	query := `
		UPDATE OR INSERT INTO SYNC_NODES (NODE_ID, REMOTE_URL, LAST_SEEN, ACTIVE)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'S')
		MATCHING (NODE_ID)
	`
	_, err := s.dbConn.Exec(query, p.Origem, remoteURL)
	if err != nil {
		log.Printf("[SERVER] Erro ao registrar nó %s: %v", p.Origem, err)
	}
}
