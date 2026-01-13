# Agentes do Projeto

Este arquivo define os papéis e responsabilidades dos agentes envolvidos no desenvolvimento e execução do sistema de sincronização.

## 1. Persona de Desenvolvimento (Engenheiro Sênior)
Responsável por garantir que a implementação siga as melhores práticas de Go e sistemas distribuídos.

- **Foco**: Resiliência, Performance, Idempotência e Código Limpo.
- **Especialidade**: Firebird 2.5, CDC (Change Data Capture) e Integrações Bidirecionais.
- **Diretriz**: "Nenhuma operação de outro banco deve ser integrada; nenhum loop deve ocorrer."

## 2. Go Sync Agent (Software)
O componente executável que rodará em ambos os nós (Node A e Node B).

### Responsabilidades Críticas:
- **Monitoramento**: Executar o `fbtracemgr` e realizar o parsing do stream de saída em tempo real.
- **Mapeamento de Contexto**: Manter o mapa `connection_id → database_path` via eventos `ATTACH_DATABASE`.
- **Filtro Anti-Loop**: Ignorar operações originadas por conexões com Application Name = `FB_SYNC_AGENT`.
- **Persistência**: Garantir que todo evento capturado pós-commit seja registrado na `FILA_INTEGRACAO`.
- **Comunicação**: Enviar e receber eventos via Webhook HTTPS com autenticação por token.

### Garantias Técnicas:
- **Idempotência**: Uso de `EVENT_ID` (UUID) para evitar duplicidade na aplicação dos dados.
- **Resiliência**: Mecanismo de retry com exponential backoff para falhas de rede.
- **Auditabilidade**: Status detalhado na tabela de fila para acompanhamento de falhas.

## 3. Fluxo de Decisão do Agente
1. Captura evento via Trace API.
2. Identifica se o Database Path é o configurado.
3. Verifica se o App Name é do próprio agente (se sim, ignora).
4. Aguarda o COMMIT da transação.
5. Snapshot dos dados (Select por PK).
6. Registra na Fila como Pendente.
7. Webhook Client tenta o envio.
8. Webhook Server remoto aplica os dados e retorna ACK.
9. Atualiza status na fila para Enviado/Aplicado.
