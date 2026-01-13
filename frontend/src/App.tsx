import { useState, useEffect } from 'react';
import { Database, Cloud, Settings, Activity, ShieldCheck, FolderOpen, Play, RotateCcw } from 'lucide-react';
import './style.css';
import { TestFirebirdConnection, SelectDatabaseFile, BuildDSN } from "../wailsjs/go/ui/App";

function App() {
    const [dbPath, setDbPath] = useState("C:\\dados\\TESTE.fb");
    const [dbUser, setDbUser] = useState("SYSDBA");
    const [dbPass, setDbPass] = useState("masterkey");
    const [status, setStatus] = useState({ fb: 'offline', relay: 'offline', svc: 'stopped' });
    const [logs, setLogs] = useState<{ t: string, m: string }[]>([]);
    const [testResult, setTestResult] = useState("");

    const addLog = (m: string) => {
        setLogs(prev => [{ t: new Date().toLocaleTimeString(), m }, ...prev].slice(0, 50));
    };

    const handleBrowse = async () => {
        const file = await SelectDatabaseFile();
        if (file) {
            setDbPath(file);
            addLog(`Banco selecionado: ${file}`);
        }
    };

    const handleTest = async () => {
        addLog("Testando conexão com Firebird...");
        try {
            // Constrói DSN normalizada com WIN1252
            const dsn = await BuildDSN(dbPath, dbUser, dbPass);
            const result = await TestFirebirdConnection(dsn);
            setTestResult(result);
            if (result.includes("sucesso")) {
                setStatus(s => ({ ...s, fb: 'online' }));
                addLog("Firebird conectado com Charset WIN1252.");
            } else {
                setStatus(s => ({ ...s, fb: 'offline' }));
                addLog("Falha na conexão Firebird.");
            }
        } catch (e) {
            setTestResult("Erro ao chamar bridge: " + e);
        }
    };

    return (
        <div id="app">
            <div className="dashboard-container">
                <aside className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '1rem' }}>
                        <div style={{ background: 'var(--primary)', padding: '8px', borderRadius: '10px' }}>
                            <Settings size={24} color="white" />
                        </div>
                        <h2>Configurações</h2>
                    </div>

                    <div className="form-group">
                        <label>CAMINHO DO BANCO (.FB / .FDB)</label>
                        <div style={{ display: 'flex', gap: '8px' }}>
                            <input value={dbPath} onChange={e => setDbPath(e.target.value)} placeholder="C:\caminho\banco.fdb" />
                            <button className="icon-btn" onClick={handleBrowse} title="Procurar arquivo">
                                <FolderOpen size={18} />
                            </button>
                        </div>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '15px' }}>
                        <div className="form-group">
                            <label>USUÁRIO</label>
                            <input value={dbUser} onChange={e => setDbUser(e.target.value)} />
                        </div>
                        <div className="form-group">
                            <label>SENHA</label>
                            <input type="password" value={dbPass} onChange={e => setDbPass(e.target.value)} />
                        </div>
                    </div>

                    <div className="form-group">
                        <label>NODE ID</label>
                        <input defaultValue="LOJA_VITORIA" />
                    </div>

                    <div className="form-group">
                        <label>RELAY CLOUD URL</label>
                        <input defaultValue="wss://relay.firechat.cloud" />
                    </div>

                    <button className="primary" onClick={handleTest}>TESTAR CONEXÃO</button>

                    <div style={{ marginTop: 'auto', borderTop: '1px solid var(--glass-border)', paddingTop: '1rem' }}>
                        <button className="primary" onClick={() => addLog("Iniciando serviço...")} style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}>
                            <Play size={18} /> INICIAR SERVIÇO
                        </button>
                    </div>
                </aside>

                <main style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
                    <div className="card" style={{ display: 'flex', justifyContent: 'space-around', padding: '1rem' }}>
                        <div style={{ textAlign: 'center' }}>
                            <div style={{ color: 'var(--text-dim)', fontSize: '0.8rem', marginBottom: '5px' }}>Firebird Database</div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center' }}>
                                <Database size={18} color={status.fb === 'online' ? 'var(--success)' : 'var(--error)'} />
                                <span className={`status-badge ${status.fb === 'online' ? 'status-online' : 'status-offline'}`}>
                                    {status.fb}
                                </span>
                            </div>
                        </div>
                        <div style={{ textAlign: 'center' }}>
                            <div style={{ color: 'var(--text-dim)', fontSize: '0.8rem', marginBottom: '5px' }}>Relay Tunnel</div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center' }}>
                                <Cloud size={18} color={status.relay === 'online' ? 'var(--success)' : 'var(--error)'} />
                                <span className={`status-badge ${status.relay === 'online' ? 'status-online' : 'status-offline'}`}>
                                    {status.relay}
                                </span>
                            </div>
                        </div>
                        <div style={{ textAlign: 'center' }}>
                            <div style={{ color: 'var(--text-dim)', fontSize: '0.8rem', marginBottom: '5px' }}>Sync Service</div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center' }}>
                                <Activity size={18} color={status.svc === 'online' ? 'var(--success)' : 'var(--error)'} />
                                <span className={`status-badge ${status.svc === 'online' ? 'status-online' : 'status-offline'}`}>
                                    {status.svc}
                                </span>
                            </div>
                        </div>
                    </div>

                    {testResult && (
                        <div className="card" style={{ padding: '1rem', borderLeft: '4px solid var(--primary)', display: 'flex', alignItems: 'center', gap: '10px' }}>
                            <ShieldCheck color="var(--primary)" size={24} />
                            <div>{testResult}</div>
                        </div>
                    )}

                    <div className="card" style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
                            <h3>Monitor de Atividades</h3>
                            <button onClick={() => setLogs([])} style={{ background: 'transparent', color: 'var(--text-dim)', display: 'flex', alignItems: 'center', gap: '5px' }}>
                                <RotateCcw size={14} /> Limpar
                            </button>
                        </div>
                        <div className="log-viewer">
                            {logs.length === 0 && <div style={{ color: 'var(--text-dim)', fontStyle: 'italic' }}>Aguardando atividades...</div>}
                            {logs.map((log, i) => (
                                <div key={i} className="log-entry">
                                    <span className="log-time">[{log.t}]</span> {log.m}
                                </div>
                            ))}
                        </div>
                    </div>
                </main>
            </div>
        </div>
    );
}

export default App;
