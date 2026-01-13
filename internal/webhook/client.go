package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
)

type Client struct {
	cfg *config.Config
}

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// Send envia um payload genérico para o remoto
func (c *Client) Send(ctx context.Context, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", c.cfg.Webhook.RemoteURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	var token string
	if c.cfg.Webhook.Token != "" {
		token = c.cfg.Webhook.Token
	} else {
		token = "ATS_SYNC_DEFAULT"
	}
	req.Header.Set("X-Sync-Token", token)

	timeout := c.cfg.Integracao.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor remoto retornou status %d", resp.StatusCode)
	}

	return nil
}
