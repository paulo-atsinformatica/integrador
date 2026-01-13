package ui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/atsinformatica/firebird-sync-agent/internal/config"
	_ "github.com/nakagami/firebirdsql"
)

type ConfigParams struct {
	NodeID          string   `json:"node_id"`
	DBPath          string   `json:"db_path"`
	IsClient        bool     `json:"is_client"`
	RemoteURL       string   `json:"remote_url"`
	Tables          []string `json:"tables"`
	IntervalSeconds int      `json:"interval_seconds"`
}

type UIServer struct {
	cfg *config.Config
}

func NewUIServer(cfg *config.Config) *UIServer {
	return &UIServer{cfg: cfg}
}

func (s *UIServer) Start(port string) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/api/list-tables", s.handleListTables)
	http.HandleFunc("/api/save", s.handleSaveConfig)

	log.Printf("UI de Configuração rodando em http://localhost%s\n", port)
	return http.ListenAndServe(port, nil)
}

func (s *UIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Configuração Agente Firebird</title>
    <style>
        body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
        .form-group { margin-bottom: 1rem; }
        label { display: block; font-weight: bold; margin-bottom: 0.5rem; }
        input[type="text"], input[type="number"] { width: 100%; padding: 0.5rem; }
        .tables-list { max-height: 300px; overflow-y: auto; border: 1px solid #ccc; padding: 0.5rem; }
        button { padding: 1rem 2rem; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        .status { margin-top: 1rem; padding: 1rem; display: none; }
        .success { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
    </style>
</head>
<body>
    <h1>Configuração do Agente</h1>
    
    <div class="form-group">
        <label>Tipo de Nó</label>
        <select id="nodeType" onchange="toggleClientConfig()">
            <option value="central">Centralizador</option>
            <option value="client">Cliente</option>
        </select>
    </div>

    <div class="form-group">
        <label>Caminho do Banco (DSN Firebird)</label>
        <div style="display: flex; gap: 0.5rem;">
            <input type="text" id="dbPath" value="SYSDBA:masterkey@localhost:3050/C:/dados/banco.fdb" placeholder="user:pass@host/path">
            <button type="button" onclick="loadTables()" style="padding: 0.5rem;">Listar Tabelas</button>
        </div>
    </div>

    <div class="form-group" id="clientConfig" style="display:none;">
        <label>ID da Conexão (Nome único deste cliente)</label>
        <input type="text" id="nodeID" placeholder="EX: LOJA_01">
    </div>

    <div class="form-group">
        <label>URL do Remoto (Webhook)</label>
        <input type="text" id="remoteURL" value="http://localhost:8080/sync">
    </div>

    <div class="form-group">
        <label>Intervalo de Envio (segundos)</label>
        <input type="number" id="interval" value="30">
    </div>

    <div class="form-group">
        <label>Tabelas para Integrar</label>
        <div id="tablesList" class="tables-list">
            <p style="color: #666;">Clique em "Listar Tabelas" para carregar...</p>
        </div>
    </div>

    <button onclick="saveConfig()">Salvar e Instalar Serviço</button>

    <div id="statusMsg" class="status"></div>

    <script>
        function toggleClientConfig() {
            const isClient = document.getElementById('nodeType').value === 'client';
            document.getElementById('clientConfig').style.display = isClient ? 'block' : 'none';
        }

        async function loadTables() {
            const dsn = document.getElementById('dbPath').value;
            const list = document.getElementById('tablesList');
            list.innerHTML = 'Carregando...';
            
            try {
                const res = await fetch('/api/list-tables?dsn=' + encodeURIComponent(dsn));
                const data = await res.json();
                
                if (data.error) throw data.error;

                list.innerHTML = '';
                data.tables.forEach(t => {
                    const div = document.createElement('div');
                    div.innerHTML = '<label style="font-weight: normal;"><input type="checkbox" name="tables" value="'+t+'"> '+t+'</label>';
                    list.appendChild(div);
                });
            } catch (e) {
                list.innerHTML = '<p class="error">Erro: ' + e + '</p>';
            }
        }

        async function saveConfig() {
            const status = document.getElementById('statusMsg');
            status.style.display = 'none';
            status.className = 'status';

            const tables = Array.from(document.querySelectorAll('input[name="tables"]:checked')).map(cb => cb.value);
            
            const payload = {
                node_id: document.getElementById('nodeID').value || 'CENTRAL',
                db_path: document.getElementById('dbPath').value,
                is_client: document.getElementById('nodeType').value === 'client',
                remote_url: document.getElementById('remoteURL').value,
                interval_seconds: parseInt(document.getElementById('interval').value),
                tables: tables
            };

            try {
                const res = await fetch('/api/save', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });
                const data = await res.json();
                if (data.error) throw data.error;

                status.textContent = 'Configuração salva e serviço instalado com sucesso!';
                status.className = 'status success';
                status.style.display = 'block';
            } catch (e) {
                status.textContent = 'Erro ao salvar: ' + e;
                status.className = 'status error';
                status.style.display = 'block';
            }
        }
    </script>
</body>
</html>
	`
	w.Write([]byte(tmpl))
}

func (s *UIServer) handleListTables(w http.ResponseWriter, r *http.Request) {
	dsn := r.URL.Query().Get("dsn")
	if dsn == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "DSN obrigatório"})
		return
	}

	if !strings.Contains(dsn, "charset=") {
		dsn += "?charset=WIN1252"
	}

	dbConn, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	defer dbConn.Close()

	query := `
		SELECT TRIM(RDB$RELATION_NAME) 
		FROM RDB$RELATIONS 
		WHERE RDB$SYSTEM_FLAG = 0 AND RDB$VIEW_BLR IS NULL
		ORDER BY RDB$RELATION_NAME
	`
	rows, err := dbConn.Query(query)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro SQL: " + err.Error()})
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			tables = append(tables, name)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"tables": tables})
}

func (s *UIServer) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var p ConfigParams
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	cfgContent := fmt.Sprintf(`node_id: "%s"
firebird:
  dsn: "%s"
  app_name: "FB_SYNC_AGENT"
  
webhook:
  listen_addr: ":8080"
  remote_url: "%s"
  token: "ATS_SECURE_TOKEN"

integracao:
  retry_interval_seconds: %d
  batch_size: 50
  max_retries: 5
  timeout_seconds: 10
`, p.NodeID, p.DBPath, p.RemoteURL, p.IntervalSeconds)

	if err := os.WriteFile("config.yaml", []byte(cfgContent), 0644); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro ao gravar config.yaml: " + err.Error()})
		return
	}

	dsn := p.DBPath
	if !strings.Contains(dsn, "charset=") {
		dsn += "?charset=WIN1252"
	}
	dbConn, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro ao conectar DB para triggers: " + err.Error()})
		return
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec("CREATE TABLE TABELAS_INTEGRADAS (NOME_TABELA VARCHAR(31) NOT NULL PRIMARY KEY, ATIVO CHAR(1) DEFAULT 'S')"); err != nil {
		log.Printf("[DEBUG] Tabela TABELAS_INTEGRADAS já existe ou erro: %v", err)
	}

	tx, err := dbConn.Begin()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro transação: " + err.Error()})
		return
	}
	tx.Exec("DELETE FROM TABELAS_INTEGRADAS")
	for _, t := range p.Tables {
		tx.Exec("INSERT INTO TABELAS_INTEGRADAS (NOME_TABELA, ATIVO) VALUES (?, 'S')", t)
	}
	if err := tx.Commit(); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro commit tabelas: " + err.Error()})
		return
	}

	if err := InstallTriggers(dbConn, p.Tables); err != nil {
		log.Printf("[ERROR] Falha ao instalar triggers: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Erro ao criar triggers: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// InstallTriggers garante a existência da tabela de fila e instala as triggers nas tabelas selecionadas
func InstallTriggers(db *sql.DB, tables []string) error {
	log.Println("[INFO] Verificando/Criando tabela FILA_INTEGRACAO...")
	_, err := db.Exec(`CREATE TABLE FILA_INTEGRACAO (
    ID INTEGER NOT NULL PRIMARY KEY,
    EVENT_ID CHAR(36) NOT NULL,
    TABELA VARCHAR(31) NOT NULL,
    OPERACAO CHAR(1) NOT NULL,
    PK_JSON BLOB SUB_TYPE TEXT,
    PAYLOAD_JSON BLOB SUB_TYPE TEXT,
    ORIGEM VARCHAR(20),
    STATUS CHAR(1) DEFAULT 'P',
    TENTATIVAS INTEGER DEFAULT 0,
    DT_EVENTO TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    DT_ULT_ENVIO TIMESTAMP,
    ERRO_MSG BLOB SUB_TYPE TEXT
	)`)
	if err != nil {
		log.Printf("[DEBUG] Nota: FILA_INTEGRACAO pode já existir: %v", err)
	}

	if _, err := db.Exec("CREATE GENERATOR GEN_FILA_INTEGRACAO_ID"); err != nil {
		log.Printf("[DEBUG] Nota: Generator pode já existir: %v", err)
	}

	_, err = db.Exec(`CREATE TRIGGER TRG_FILA_INTEGRACAO_BI FOR FILA_INTEGRACAO ACTIVE BEFORE INSERT POSITION 0 AS BEGIN 
		IF (NEW.ID IS NULL) THEN NEW.ID = GEN_ID(GEN_FILA_INTEGRACAO_ID, 1); END`)
	if err != nil {
		log.Printf("[DEBUG] Nota: Trigger BI pode já existir: %v", err)
	}

	// Tabelas para Multi-Cliente (Broadcast)
	_, _ = db.Exec(`CREATE TABLE SYNC_NODES (
		NODE_ID VARCHAR(20) NOT NULL PRIMARY KEY,
		NODE_NAME VARCHAR(100),
		REMOTE_URL VARCHAR(255) NOT NULL,
		LAST_SEEN TIMESTAMP,
		ACTIVE CHAR(1) DEFAULT 'S' CHECK (ACTIVE IN ('S', 'N'))
	)`)

	_, _ = db.Exec(`CREATE TABLE FILA_DESTINOS (
		ID INTEGER NOT NULL PRIMARY KEY,
		FILA_ID INTEGER NOT NULL,
		NODE_ID VARCHAR(20) NOT NULL,
		STATUS CHAR(1) DEFAULT 'P',
		TENTATIVAS INTEGER DEFAULT 0,
		ERRO_MSG BLOB SUB_TYPE TEXT,
		DT_ULT_TENTATIVA TIMESTAMP
	)`)

	_, _ = db.Exec("CREATE GENERATOR GEN_FILA_DESTINOS_ID")

	_, _ = db.Exec(`CREATE TRIGGER TRG_FILA_DESTINOS_BI FOR FILA_DESTINOS ACTIVE BEFORE INSERT POSITION 0 AS BEGIN 
		IF (NEW.ID IS NULL) THEN NEW.ID = GEN_ID(GEN_FILA_DESTINOS_ID, 1); END`)

	// Campos técnicos que o ERP muda e NÃO devem disparar nova captura
	ignoreFields := map[string]bool{
		"FLAGINTEGRACAO":      true,
		"SINCRONIZADO":        true,
		"SYNC_TS":             true,
		"DATA_ULT_ALTERACAO":  true,
		"HORA_ULT_ALTERACAO":  true,
		"DATA_ULT_SINCRONIZA": true,
		"HORA_ULT_SINCRONIZA": true,
		"VERSION":             true,
		"USUARIO_ALT":         true,
	}

	for _, tableName := range tables {
		rows, err := db.Query(`SELECT TRIM(R.RDB$FIELD_NAME) FROM RDB$RELATION_FIELDS R WHERE R.RDB$RELATION_NAME = ? ORDER BY R.RDB$FIELD_POSITION`, tableName)
		if err != nil {
			return err
		}

		var cols []string
		for rows.Next() {
			var c string
			rows.Scan(&c)
			cols = append(cols, c)
		}
		rows.Close()

		if len(cols) == 0 {
			continue
		}

		// Detecção de mudanças usando IS DISTINCT FROM (mais robusto no FB 2.5)
		var changeChecks []string
		for _, col := range cols {
			if ignoreFields[strings.ToUpper(col)] {
				continue
			}
			// IS DISTINCT FROM trata NULLs automaticamente
			check := fmt.Sprintf("(OLD.%[1]s IS DISTINCT FROM NEW.%[1]s)", col)
			changeChecks = append(changeChecks, check)
		}
		changeCondition := strings.Join(changeChecks, " OR ")

		if changeCondition == "" {
			changeCondition = "1=1"
		}

		pkColRows, err := db.Query(`
			SELECT TRIM(s.RDB$FIELD_NAME)
			FROM RDB$RELATION_CONSTRAINTS c
			JOIN RDB$INDEX_SEGMENTS s ON c.RDB$INDEX_NAME = s.RDB$INDEX_NAME
			WHERE c.RDB$CONSTRAINT_TYPE = 'PRIMARY KEY' 
			  AND c.RDB$RELATION_NAME = ?
			ORDER BY s.RDB$FIELD_POSITION`, tableName)
		if err != nil {
			return err
		}
		var pkCols []string
		for pkColRows.Next() {
			var c string
			pkColRows.Scan(&c)
			pkCols = append(pkCols, c)
		}
		pkColRows.Close()

		if len(pkCols) == 0 {
			uniRows, _ := db.Query(`
				SELECT FIRST 1 TRIM(s.RDB$FIELD_NAME)
				FROM RDB$RELATION_CONSTRAINTS c
				JOIN RDB$INDEX_SEGMENTS s ON c.RDB$INDEX_NAME = s.RDB$INDEX_NAME
				WHERE c.RDB$CONSTRAINT_TYPE = 'UNIQUE' 
				  AND c.RDB$RELATION_NAME = ?
				ORDER BY c.RDB$CONSTRAINT_NAME, s.RDB$FIELD_POSITION`, tableName)
			if uniRows != nil {
				for uniRows.Next() {
					var c string
					uniRows.Scan(&c)
					pkCols = append(pkCols, c)
				}
				uniRows.Close()
			}
		}

		if len(pkCols) == 0 && len(cols) > 0 {
			pkCols = append(pkCols, cols[0])
		}

		var jsonParts []string
		for i, col := range cols {
			part := fmt.Sprintf(" '\"%s\": \"' || COALESCE(CAST(NEW.%s AS VARCHAR(200)), '') || '\"'", col, col)
			jsonParts = append(jsonParts, part)
			if i < len(cols)-1 {
				jsonParts = append(jsonParts, " || ',' || ")
			}
		}
		jsonPayload := strings.Join(jsonParts, "")

		var pkOldParts []string
		var pkNewParts []string
		for i, col := range pkCols {
			pkOldParts = append(pkOldParts, fmt.Sprintf(" '\"%s\": \"' || COALESCE(CAST(OLD.%s AS VARCHAR(100)), '') || '\"'", col, col))
			pkNewParts = append(pkNewParts, fmt.Sprintf(" '\"%s\": \"' || COALESCE(CAST(NEW.%s AS VARCHAR(100)), '') || '\"'", col, col))
			if i < len(pkCols)-1 {
				pkOldParts = append(pkOldParts, " || ',' || ")
				pkNewParts = append(pkNewParts, " || ',' || ")
			}
		}
		pkOldJSON := strings.Join(pkOldParts, "")
		pkNewJSON := strings.Join(pkNewParts, "")

		triggerName := "TRG_SYNC_" + tableName
		if len(triggerName) > 31 {
			triggerName = triggerName[:31]
		}

		sql := fmt.Sprintf(`
		CREATE OR ALTER TRIGGER %s FOR %s
		ACTIVE AFTER INSERT OR UPDATE OR DELETE POSITION 100
		AS
		DECLARE VARIABLE OP CHAR(1);
		DECLARE VARIABLE PAYLOAD BLOB SUB_TYPE TEXT;
		DECLARE VARIABLE PK_VAL VARCHAR(1000);
		BEGIN
			IF (UPDATING) THEN
			BEGIN
				-- Idempotência: só entra se algo útil mudou
				IF (NOT (%s)) THEN EXIT;
			END

			IF (INSERTING) THEN OP = 'I';
			ELSE IF (UPDATING) THEN OP = 'U';
			ELSE OP = 'D';

			IF (OP IN ('I', 'U')) THEN
				PAYLOAD = '{' || %s || '}';
			ELSE
				PAYLOAD = NULL;

			IF (OP = 'D') THEN PK_VAL = '{' || %s || '}';
			ELSE PK_VAL = '{' || %s || '}';

			INSERT INTO FILA_INTEGRACAO (EVENT_ID, TABELA, OPERACAO, PK_JSON, PAYLOAD_JSON, ORIGEM)
			VALUES (UUID_TO_CHAR(GEN_UUID()), '%s', :OP, :PK_VAL, :PAYLOAD, 'TRIGGER');
		END
		`, triggerName, tableName, changeCondition, jsonPayload, pkOldJSON, pkNewJSON, tableName)

		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("erro trigger %s: %v", tableName, err)
		}
	}
	return nil
}
