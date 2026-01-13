package webhook

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"time"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/models"
	"github.com/gorilla/websocket"
)

// RelayMessage deve ser idêntico ao do Hub
type RelayMessage struct {
	TargetNode string          `json:"target"`
	SourceNode string          `json:"source"`
	Payload    json.RawMessage `json:"payload"`
	Type       string          `json:"type"`
}

type RelayClient struct {
	cfg     *config.Config
	handler *Server
	conn    *websocket.Conn
	send    chan RelayMessage
}

func NewRelayClient(cfg *config.Config, handler *Server) *RelayClient {
	return &RelayClient{
		cfg:     cfg,
		handler: handler,
		send:    make(chan RelayMessage, 100),
	}
}

func (c *RelayClient) Start(ctx context.Context) {
	for {
		err := c.connectAndListen(ctx)
		if err != nil {
			log.Printf("[RELAY] Erro na conexão: %v. Reconectando em 5s...", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			continue
		}
	}
}

func (c *RelayClient) connectAndListen(ctx context.Context) error {
	u, err := url.Parse(c.cfg.Relay.HubURL)
	if err != nil {
		return err
	}

	q := u.Query()
	q.Set("node_id", c.cfg.NodeID)
	q.Set("token", c.cfg.Relay.Token)
	u.RawQuery = q.Encode()

	log.Printf("[RELAY] Conectando ao Hub em %s...", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.conn = conn
	defer c.conn.Close()

	// Goroutine de envio
	go func() {
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				data, _ := json.Marshal(msg)
				if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("[RELAY] Erro ao enviar mensagem: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Loop de recebimento
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}

		var relayMsg RelayMessage
		if err := json.Unmarshal(message, &relayMsg); err != nil {
			log.Printf("[RELAY] Erro ao decodificar mensagem do Relay: %v", err)
			continue
		}

		if relayMsg.Type == "sync" {
			var payload models.SyncPayload
			if err := json.Unmarshal(relayMsg.Payload, &payload); err != nil {
				log.Printf("[RELAY] Erro ao decodificar sync payload: %v", err)
				continue
			}

			// Processa o dado como se tivesse vindo do Webhook HTTP
			go func() {
				if err := c.handler.ProcessPayload(payload); err != nil {
					log.Printf("[RELAY] Erro ao processar sync do Relay: %v", err)
				}
			}()
		}
	}
}

func (c *RelayClient) SendSync(targetNode string, payload models.SyncPayload) {
	data, _ := json.Marshal(payload)
	c.send <- RelayMessage{
		TargetNode: targetNode,
		SourceNode: c.cfg.NodeID,
		Payload:    data,
		Type:       "sync",
	}
}
