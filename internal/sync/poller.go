package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/db"
	"github.com/atsinformatica/firebird-sync-agent/internal/models"
	"github.com/atsinformatica/firebird-sync-agent/internal/webhook"
)

// WebhookSender define a interface para o envio de dados
type WebhookSender interface {
	Send(ctx context.Context, payload interface{}) error
}

type Poller struct {
	cfg    *config.Config
	queue  *db.QueueManager
	sender WebhookSender
	relay  *webhook.RelayClient
	ctx    context.Context
	cancel context.CancelFunc
}

func NewPoller(cfg *config.Config, queue *db.QueueManager, sender WebhookSender, relay *webhook.RelayClient) *Poller {
	return &Poller{
		cfg:    cfg,
		queue:  queue,
		sender: sender,
		relay:  relay,
	}
}

func (p *Poller) Start(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)

	interval := p.cfg.Integracao.RetryIntervalSeconds
	if interval <= 0 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	log.Printf("[POLLER] Iniciando monitoramento da fila (Intervalo: %ds)\n", interval)

	for {
		select {
		case <-p.ctx.Done():
			log.Println("[POLLER] Parando monitoramento...")
			return
		case <-ticker.C:
			p.processQueue()
		}
	}
}

func (p *Poller) processQueue() {
	p.dispatchEvents()
	p.sendEvents()
}

func (p *Poller) dispatchEvents() {
	items, err := p.queue.GetPending(p.cfg.Integracao.BatchSize)
	if err != nil {
		log.Printf("[POLLER] Erro ao buscar pendências para despacho: %v", err)
		return
	}

	if len(items) == 0 {
		return
	}

	nodes, err := p.queue.GetActiveNodes()
	if err != nil {
		log.Printf("[POLLER] Erro ao buscar nós ativos: %v", err)
		return
	}

	for _, item := range items {
		err := p.queue.CreateDestinations(item.ID, nodes, item.Origem)
		if err != nil {
			log.Printf("[POLLER] Erro ao criar destinos para ID %d: %v", item.ID, err)
			continue
		}
		p.queue.UpdateStatus(item.ID, "D", "Evento despachado para nós ativos")
	}
}

func (p *Poller) sendEvents() {
	dests, err := p.queue.GetPendingDestinations(p.cfg.Integracao.BatchSize)
	if err != nil {
		log.Printf("[POLLER] Erro ao buscar tarefas de envio: %v", err)
		return
	}

	if len(dests) == 0 {
		return
	}

	nodeTasks := make(map[string][]*db.FilaDestino)
	for _, d := range dests {
		nodeTasks[d.NodeID] = append(nodeTasks[d.NodeID], d)
	}

	nodes, _ := p.queue.GetActiveNodes()
	nodeURLs := make(map[string]string)
	for _, n := range nodes {
		nodeURLs[n.NodeID] = n.RemoteURL
	}

	for nodeID, tasks := range nodeTasks {
		remoteURL := nodeURLs[nodeID]
		if remoteURL == "" && !p.cfg.Relay.Enabled {
			continue
		}

		for _, task := range tasks {
			item := task.Item

			var pkMap map[string]interface{}
			var payloadMap map[string]interface{}

			json.Unmarshal([]byte(item.PKJSON), &pkMap)
			if len(item.PayloadJSON) > 0 {
				json.Unmarshal([]byte(item.PayloadJSON), &payloadMap)
			}

			webhookPayload := models.SyncPayload{
				EventID:     item.EventID,
				Table:       item.Tabela,
				Operation:   item.Operacao,
				PKJSON:      pkMap,
				PayloadJSON: payloadMap,
				Timestamp:   item.DTEvento,
				Origem:      p.cfg.NodeID,
			}

			// Se o Relay estiver ligado e for um nó remoto, tentamos enviar via Relay primeiro
			if p.cfg.Relay.Enabled && p.relay != nil {
				log.Printf("[POLLER] Enviando para %s via RELAY...", nodeID)
				p.relay.SendSync(nodeID, webhookPayload)
				p.queue.UpdateDestinoStatus(task.ID, "E", "")
				continue
			}

			if remoteURL == "" {
				continue
			}

			sender := &webhookSenderWithURL{
				p:   p,
				url: remoteURL,
			}

			err := sender.Send(p.ctx, webhookPayload)
			if err != nil {
				log.Printf("[POLLER] Falha ao enviar para %s (ID %d): %v", nodeID, task.ID, err)
				p.queue.UpdateDestinoStatus(task.ID, "R", err.Error())
				break
			} else {
				log.Printf("[POLLER] Sucesso para %s (ID %d)", nodeID, task.ID)
				p.queue.UpdateDestinoStatus(task.ID, "E", "")
			}
		}
	}
}

type webhookSenderWithURL struct {
	p   *Poller
	url string
}

func (s *webhookSenderWithURL) Send(ctx context.Context, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	token := s.p.cfg.Webhook.Token
	if token == "" {
		token = "ATS_SYNC_DEFAULT"
	}
	req.Header.Set("X-Sync-Token", token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyErr, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(bodyErr))
	}
	return nil
}
