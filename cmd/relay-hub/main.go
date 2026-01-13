package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	addr  = flag.String("addr", ":8000", "Endereço de escuta do Relay")
	token = flag.String("token", "ATS_RELAY_SECRET", "Token de autenticação do Relay")
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Em produção, validar origem
}

// RelayMessage define o envelope usado no túnel
type RelayMessage struct {
	TargetNode string          `json:"target"`
	SourceNode string          `json:"source"`
	Payload    json.RawMessage `json:"payload"`
	Type       string          `json:"type"` // e.g., "sync", "command", "ping"
}

type Hub struct {
	nodes sync.Map // map[string]*websocket.Conn
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Validar Token
	if r.Header.Get("X-Relay-Token") != *token && r.URL.Query().Get("token") != *token {
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	// 2. Obter NodeID
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "node_id é obrigatório", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[RELAY] Erro no upgrade do nó %s: %v", nodeID, err)
		return
	}
	defer conn.Close()

	h.nodes.Store(nodeID, conn)
	log.Printf("[RELAY] Nó Conectado: %s", nodeID)
	defer func() {
		h.nodes.Delete(nodeID)
		log.Printf("[RELAY] Nó Desconectado: %s", nodeID)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[RELAY] Erro de leitura do nó %s: %v", nodeID, err)
			break
		}

		var relayMsg RelayMessage
		if err := json.Unmarshal(message, &relayMsg); err != nil {
			log.Printf("[RELAY] Payload inválido do nó %s", nodeID)
			continue
		}

		// Preenche a origem se estiver vazia para segurança
		if relayMsg.SourceNode == "" {
			relayMsg.SourceNode = nodeID
		}

		// Roteamento
		if targetConn, ok := h.nodes.Load(relayMsg.TargetNode); ok {
			wsTarget := targetConn.(*websocket.Conn)
			msgOut, _ := json.Marshal(relayMsg)
			if err := wsTarget.WriteMessage(websocket.TextMessage, msgOut); err != nil {
				log.Printf("[RELAY] Erro ao enviar para %s: %v", relayMsg.TargetNode, err)
			}
		} else {
			log.Printf("[RELAY] Destino não encontrado: %s (De: %s)", relayMsg.TargetNode, nodeID)
			// Opcional: Responder ao emissor informando que o alvo está offline
		}
	}
}

func main() {
	flag.Parse()

	// Prioriza variáveis de ambiente (padrão em Docker/Coolify)
	envAddr := os.Getenv("PORT")
	if envAddr == "" {
		envAddr = "8080" // Padrão se não informado
	}
	listenAddr := ":" + envAddr

	envToken := os.Getenv("RELAY_TOKEN")
	if envToken != "" {
		*token = envToken
	}

	hub := &Hub{}

	log.Printf("[RELAY] Iniciando Relay Hub em %s (Token: %s)...", listenAddr, *token)
	log.Fatal(http.ListenAndServe(listenAddr, hub))
}
