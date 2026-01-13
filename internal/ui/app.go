package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	"github.com/atsinformatica/firebird-sync-agent/internal/db"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// TestFirebirdConnection tenta conectar ao banco e retorna sucesso ou erro
func (a *App) TestFirebirdConnection(dsn string) string {
	conn, err := db.Connect(dsn)
	if err != nil {
		return fmt.Sprintf("Erro de conexão: %v", err)
	}
	defer conn.Close()
	return "Conexão com Firebird estabelecida com sucesso!"
}

// GetCurrentConfig retorna a configuração atual se existir
func (a *App) GetCurrentConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

// SelectDatabaseFile abre o seletor de arquivos do Windows
func (a *App) SelectDatabaseFile() string {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecionar Banco de Dados Firebird",
		Filters: []runtime.FileFilter{
			{DisplayName: "Bancos Firebird (*.fb;*.fdb)", Pattern: "*.fb;*.fdb"},
		},
	})
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(file, "/", "\\")
}

// BuildDSN monta a string de conexão normalizada
func (a *App) BuildDSN(path, user, pass string) string {
	normalizedPath := strings.ReplaceAll(path, "/", "\\")
	// Exemplo: user:pass@localhost:3050/c:\path\banco.fb?charset=WIN1252
	return fmt.Sprintf("%s:%s@localhost:3050/%s?charset=WIN1252", user, pass, normalizedPath)
}

// SaveConfig salva a configuração no arquivo YAML
func (a *App) SaveConfig(cfg *config.Config, path string) error {
	return cfg.Save(path)
}
