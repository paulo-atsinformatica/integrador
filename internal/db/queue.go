package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type FilaItem struct {
	ID          int64
	EventID     string
	Tabela      string
	Operacao    string
	PKJSON      string
	PayloadJSON string
	Origem      string
	Status      string
	Tentativas  int
	DTEvento    time.Time
}

type Node struct {
	NodeID    string
	RemoteURL string
}

type FilaDestino struct {
	ID         int64
	FilaID     int64
	NodeID     string
	Status     string
	Tentativas int
	ERROMsg    string
	Item       *FilaItem
}

type QueueManager struct {
	db     *sql.DB
	nodeID string
}

func NewQueueManager(db *sql.DB, nodeID string) *QueueManager {
	return &QueueManager{db: db, nodeID: nodeID}
}

// Insert adiciona um novo evento na fila
func (q *QueueManager) Insert(tabela, operacao string, pk map[string]interface{}, payload map[string]interface{}) error {
	eventID := uuid.New().String()

	pkJSON, _ := json.Marshal(pk)
	payloadJSON, _ := json.Marshal(payload)

	query := `
		INSERT INTO FILA_INTEGRACAO (EVENT_ID, TABELA, OPERACAO, PK_JSON, PAYLOAD_JSON, ORIGEM, STATUS, TENTATIVAS, DT_EVENTO)
		VALUES (?, ?, ?, ?, ?, ?, 'P', 0, CURRENT_TIMESTAMP)
	`
	_, err := q.db.Exec(query, eventID, tabela, operacao, string(pkJSON), string(payloadJSON), q.nodeID)
	if err != nil {
		return fmt.Errorf("erro ao inserir na fila: %w", err)
	}

	return nil
}

// GetPending retorna itens pendentes ou para reenvio
func (q *QueueManager) GetPending(limit int) ([]*FilaItem, error) {
	query := `
		SELECT FIRST ? ID, EVENT_ID, TABELA, OPERACAO, PK_JSON, PAYLOAD_JSON, ORIGEM, STATUS, TENTATIVAS, DT_EVENTO
		FROM FILA_INTEGRACAO
		WHERE STATUS IN ('P', 'R')
		ORDER BY ID ASC
	`
	rows, err := q.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*FilaItem
	for rows.Next() {
		item := &FilaItem{}
		err := rows.Scan(
			&item.ID, &item.EventID, &item.Tabela, &item.Operacao,
			&item.PKJSON, &item.PayloadJSON, &item.Origem, &item.Status,
			&item.Tentativas, &item.DTEvento,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdateStatus atualiza o status de um item
func (q *QueueManager) UpdateStatus(id int64, status string, erroMsg string) error {
	query := `
		UPDATE FILA_INTEGRACAO 
		SET STATUS = ?, ERRO_MSG = ?, DT_ULT_ENVIO = CURRENT_TIMESTAMP, TENTATIVAS = TENTATIVAS + 1
		WHERE ID = ?
	`
	_, err := q.db.Exec(query, status, erroMsg, id)
	return err
}

// IsDuplicate verifica se o EVENT_ID já foi processado (idempotência no recebimento)
func (q *QueueManager) IsDuplicate(eventID string) (bool, error) {
	var exists int
	err := q.db.QueryRow("SELECT 1 FROM FILA_INTEGRACAO WHERE EVENT_ID = ?", eventID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return exists > 0, err
}

// Multi-Destino

func (q *QueueManager) GetActiveNodes() ([]Node, error) {
	rows, err := q.db.Query("SELECT NODE_ID, REMOTE_URL FROM SYNC_NODES WHERE ACTIVE = 'S'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.NodeID, &n.RemoteURL); err == nil {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

func (q *QueueManager) CreateDestinations(filaID int64, nodes []Node, ignoreNode string) error {
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, n := range nodes {
		if n.NodeID == ignoreNode {
			continue
		}
		_, err = tx.Exec("INSERT INTO FILA_DESTINOS (FILA_ID, NODE_ID, STATUS) VALUES (?, ?, 'P')", filaID, n.NodeID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (q *QueueManager) RegisterStaticNode(nodeID, remoteURL string) error {
	if remoteURL == "" {
		return nil
	}
	query := `
		UPDATE OR INSERT INTO SYNC_NODES (NODE_ID, REMOTE_URL, LAST_SEEN, ACTIVE)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'S')
		MATCHING (NODE_ID)
	`
	_, err := q.db.Exec(query, nodeID, remoteURL)
	return err
}

func (q *QueueManager) GetPendingDestinations(limit int) ([]*FilaDestino, error) {
	query := `
		SELECT FIRST ? d.ID, d.FILA_ID, d.NODE_ID, d.STATUS, d.TENTATIVAS,
		       f.EVENT_ID, f.TABELA, f.OPERACAO, f.PK_JSON, f.PAYLOAD_JSON, f.ORIGEM, f.DT_EVENTO
		FROM FILA_DESTINOS d
		JOIN FILA_INTEGRACAO f ON d.FILA_ID = f.ID
		WHERE d.STATUS IN ('P', 'R')
		ORDER BY d.ID ASC
	`
	rows, err := q.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dests []*FilaDestino
	for rows.Next() {
		d := &FilaDestino{Item: &FilaItem{}}
		err := rows.Scan(
			&d.ID, &d.FilaID, &d.NodeID, &d.Status, &d.Tentativas,
			&d.Item.EventID, &d.Item.Tabela, &d.Item.Operacao,
			&d.Item.PKJSON, &d.Item.PayloadJSON, &d.Item.Origem, &d.Item.DTEvento,
		)
		if err != nil {
			return nil, err
		}
		// Sincroniza o ID do item
		d.Item.ID = d.FilaID
		dests = append(dests, d)
	}
	return dests, nil
}

func (q *QueueManager) UpdateDestinoStatus(id int64, status string, erroMsg string) error {
	query := `
		UPDATE FILA_DESTINOS 
		SET STATUS = ?, ERRO_MSG = ?, DT_ULT_TENTATIVA = CURRENT_TIMESTAMP, TENTATIVAS = TENTATIVAS + 1
		WHERE ID = ?
	`
	_, err := q.db.Exec(query, status, erroMsg, id)
	return err
}
