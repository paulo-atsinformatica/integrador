package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/db"
	"github.com/atsinformatica/firebird-sync-agent/internal/sync"
	"github.com/atsinformatica/firebird-sync-agent/internal/ui"
	"github.com/atsinformatica/firebird-sync-agent/internal/webhook"
	"github.com/kardianos/service"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

var logger service.Logger

type program struct {
	exit       chan struct{}
	service    service.Service
	configPath string
}

func (p *program) Start(s service.Service) error {
	p.exit = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) run() {
	configPath := p.configPath
	if configPath == "" {
		exePath, _ := os.Executable()
		dir := filepath.Dir(exePath)
		configPath = filepath.Join(dir, "config.yaml")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Info("Configuração não encontrada: " + configPath + ". Iniciando Modo UI de Configuração na porta :8090")
		// Inicia UI
		srv := ui.NewUIServer(nil)
		if err := srv.Start(":8090"); err != nil {
			logger.Error(err)
		}
		return
	}

	// Modo Normal (Serviço)
	StartAgent(configPath)
}

func (p *program) Stop(s service.Service) error {
	close(p.exit)
	return nil
}

func main() {
	configFlag := flag.String("config", "", "Caminho para o arquivo de configuração")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Uso: %s [comando] [-config path]\n", os.Args[0])
		fmt.Println("\nComandos:")
		fmt.Println("  install    Instala como serviço Windows")
		fmt.Println("  uninstall  Remove o serviço")
		fmt.Println("  start      Inicia o serviço")
		fmt.Println("  stop       Para o serviço")
		fmt.Println("  ui         Força modo UI")
		fmt.Println("\nOpções:")
		flag.PrintDefaults()
	}

	// Parsing customizado para conviver com comandos do service
	var cmd string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cmd = os.Args[1]
		// Remove o comando dos args para o flag.Parse() funcionar
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "FirebirdSyncAgent",
		DisplayName: "Firebird Sync Agent",
		Description: "Sincronizador Bidirecional para Firebird",
		Arguments:   []string{"-config", *configFlag}, // Persiste o config se rodar como serviço
	}

	prg := &program{configPath: *configFlag}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	prg.service = s

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	if cmd != "" {
		switch cmd {
		case "install":
			err = s.Install()
			if err == nil {
				fmt.Println("Serviço instalado com sucesso!")
			} else {
				fmt.Printf("Erro ao instalar: %v\n", err)
			}
			return
		case "uninstall":
			err = s.Uninstall()
			if err == nil {
				fmt.Println("Serviço removido com sucesso.")
			}
			return
		case "start":
			err = s.Start()
			if err == nil {
				fmt.Println("Serviço iniciado.")
			}
			return
		case "stop":
			err = s.Stop()
			if err == nil {
				fmt.Println("Serviço parado.")
			}
			return
		case "ui":
			srv := ui.NewUIServer(nil)
			srv.Start(":8090")
			return
		}
	}

	// Se não for comando de serviço, inicia a UI Desktop
	app := ui.NewApp()
	err = wails.Run(&options.App{
		Title:  "Firebird Sync Agent",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func StartAgent(configPath string) {
	log.Printf("I: Carregando configuração: %s\n", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Errorf("Erro ao carregar config: %v", err)
		return
	}

	dbConn, err := db.Connect(cfg.Firebird.DSN)
	if err != nil {
		logger.Errorf("Erro conectar Firebird: %v", err)
		return
	}

	// Auto-instalação: Garante FILA e triggers básicos (CLIENTE, PRODUTO)
	log.Println("I: Verificando/Instalando Tabelas e Triggers automáticos...")
	if err := ui.InstallTriggers(dbConn, []string{"CLIENTE", "PRODUTO"}); err != nil {
		log.Printf("[WARN] Erro na autoinstalação de triggers: %v\n", err)
	}

	queue := db.NewQueueManager(dbConn, cfg.NodeID)

	// Garante que o destino na config seja registrado como um nó ativo (Estaticamente)
	if cfg.Webhook.RemoteURL != "" {
		remoteNodeID := "UPSTREAM"
		if cfg.NodeID == "CENTRAL" {
			remoteNodeID = "LOJA_DEFAULT"
		} else if cfg.NodeID == "LOJA" {
			remoteNodeID = "CENTRAL"
		}
		log.Printf("I: Registrando destino estático: %s -> %s\n", remoteNodeID, cfg.Webhook.RemoteURL)
		if err := queue.RegisterStaticNode(remoteNodeID, cfg.Webhook.RemoteURL); err != nil {
			log.Printf("[WARN] Erro ao registrar nó estático: %v\n", err)
		}
	}

	webhookServer := webhook.NewServer(dbConn, queue, cfg.Webhook.Token)
	webhookClient := webhook.NewClient(cfg)

	// Inicializa Relay se habilitado
	var relayClient *webhook.RelayClient
	if cfg.Relay.Enabled {
		log.Printf("I: Inicializando Cliente RELAY para %s\n", cfg.Relay.HubURL)
		relayClient = webhook.NewRelayClient(cfg, webhookServer)
	}

	poller := sync.NewPoller(cfg, queue, webhookClient, relayClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicia Relay em background
	if relayClient != nil {
		go relayClient.Start(ctx)
	}

	// Se houver config de UI port e NÃO for serviço, podemos rodar UI junto?
	// Por enquanto, modo agente é só agente.
	// Mas vamos respeitar a porta de escuta do webhook
	log.Printf("I: Iniciando Webhook Server em %s\n", cfg.Webhook.ListenAddr)
	go webhookServer.Listen(cfg.Webhook.ListenAddr)

	log.Printf("[POLLER] Monitorando banco...")
	go poller.Start(ctx)

	select {}
}
