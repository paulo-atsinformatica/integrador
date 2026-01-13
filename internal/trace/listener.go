package trace

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
)

type Listener struct {
	cfg    *config.Config
	parser *Parser
	ctx    context.Context
	cancel context.CancelFunc
}

func NewListener(cfg *config.Config, parser *Parser) *Listener {
	return &Listener{
		cfg:    cfg,
		parser: parser,
	}
}

// Start inicia o monitoramento (via arquivo ou via fbtracemgr)
func (l *Listener) Start(ctx context.Context) error {
	l.ctx, l.cancel = context.WithCancel(ctx)

	// SE tiver caminho de log configurado, usa modo "Tail"
	if l.cfg.Trace.LogPath != "" {
		fmt.Printf("[INFO] Modo System Audit ativo. Monitorando arquivo: %s\n", l.cfg.Trace.LogPath)
		go l.tailLogFile(l.cfg.Trace.LogPath)
		<-l.ctx.Done()
		return nil
	}

	// SENÃO, usa modo padrão (fbtracemgr)
	return l.startFBTraceMgr()
}

func (l *Listener) tailLogFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível abrir o arquivo de log: %v\n", err)
		return
	}
	defer file.Close()

	// Posiciona no fim do arquivo para ler apenas novos eventos
	file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	for {
		select {
		case <-l.ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(500 * time.Millisecond) // Aguarda novos dados
					continue
				}
				fmt.Printf("[ERRO] Erro ao ler log: %v\n", err)
				return
			}

			// Remove espaços e quebras
			line = strings.TrimRight(line, "\r\n")
			l.parser.ParseLine(line)
		}
	}
}

func (l *Listener) startFBTraceMgr() error {
	// Cria o arquivo temporário de configuração do Trace
	confPath, err := l.createTraceConfig()
	if err != nil {
		return fmt.Errorf("erro ao criar config de trace: %w", err)
	}
	defer os.Remove(confPath)

	// Extrai host e user/pass da DSN para o fbtracemgr
	args := []string{
		"-se", "localhost:service_mgr",
		"-user", "SYSDBA",
		"-password", "masterkey",
		"-start",
		"-config", confPath,
	}

	cmd := exec.CommandContext(l.ctx, l.cfg.Trace.FBTraceMgrPos, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("erro ao obter stdout do trace: %w", err)
	}
	cmd.Stderr = os.Stderr // Captura erros do fbtracemgr no terminal

	fmt.Printf("[DEBUG] Iniciando fbtracemgr com config em: %s\n", confPath)
	confContent, _ := os.ReadFile(confPath)
	fmt.Printf("[DEBUG] Conteúdo do arquivo de Trace:\n%s\n", string(confContent))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("erro ao iniciar fbtracemgr: %w", err)
	}

	go l.processOutput(stdout)

	err = cmd.Wait()
	if err != nil && l.ctx.Err() == nil {
		return fmt.Errorf("fbtracemgr parou inesperadamente: %w", err)
	}

	return nil
}

func (l *Listener) createTraceConfig() (string, error) {
	dbPath := l.extractPathFromDSN(l.cfg.Firebird.DSN)

	escapedPath := strings.ReplaceAll(dbPath, "\\", "[\\\\/]")
	escapedPath = strings.ReplaceAll(escapedPath, "/", "[\\\\/]")

	content := fmt.Sprintf(`
<database .*>
	enabled                   true
	log_connections           true
	log_transactions          true
	log_statement_prepare     true
	log_statement_free        true
	log_statement_start       true
	log_statement_finish      true
	log_procedure_start       true
	log_procedure_finish      true
	log_trigger_start         true
	log_trigger_finish        true
	print_plan                true
	print_perf                true
	log_blr_requests          true
	print_blr                 true
	log_dyn_requests          true
	print_dyn                 true
	time_threshold            0
</database>
`)

	tmpFile, err := os.CreateTemp("", "fbtrace_*.conf")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

func (l *Listener) processOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		l.parser.ParseLine(line)
	}
}

func (l *Listener) extractPathFromDSN(dsn string) string {
	parts := strings.Split(dsn, "?")
	pathPart := parts[0]

	lastSlash := strings.LastIndex(pathPart, "/")
	if lastSlash == -1 {
		lastSlash = strings.LastIndex(pathPart, "\\")
	}
	if lastSlash != -1 {
		return pathPart[lastSlash+1:]
	}
	return filepath.Base(pathPart)
}
