# Guia de Deploy: Relay Hub no Coolify

O **Relay Hub** é o servidor central que permite a conexão entre a Loja e a Central sem precisar de IP externo. Siga os passos abaixo para subir no seu Coolify:

---

### Opção A: Deploy via Imagem (GHCR) - MAIS RÁPIDO 🚀
Agora que configuramos o **GitHub Actions**, você pode puxar a imagem pronta:
1. No Coolify, selecione **"Docker Image"** em vez de "Application".
2. Use o endereço: `ghcr.io/paulo-atsinformatica/relay-hub:latest`
3. Se o seu repositório for privado, você precisará adicionar o seu **GitHub Personal Access Token** no Coolify.

---

### Opção B: Deploy via Código Fonte (Coolify Build)
1. No seu Dashboard do Coolify, clique em **"Create New Resource"** -> **"Application"**.
2. Selecione o seu repositório do projeto `integrador`.
3. Na configuração do Build, mude para:
   - **Build Pack:** Dockerfile
   - **Dockerfile Path:** `./Dockerfile.relay`
4. Na aba **"Environment Variables"**, adicione:
   - `RELAY_TOKEN`: Escolha uma senha forte (ex: `MinhaSenhaSuperSecreta123`).
   - `PORT`: `8080` (Opcional, padrão é 8080).

### 3. Domínio e HTTPS
- Configure um domínio ou subdomínio (ex: `relay.seuerp.com.br`).
- O Coolify gerenciará o certificado SSL automaticamente.
- **Importante:** O endereço final para configurar no Agente Desktop será `wss://relay.seuerp.com.br` (o prefixo `wss://` indica WebSocket Seguro).

### 4. Como configurar no Agente Desktop
No Dashboard WOW (V2):
1. No campo **RELAY CLOUD URL**, coloque o seu domínio: `wss://relay.seuerp.com.br`.
2. Clique em **TOKEN DE SINCRONIZAÇÃO** (ou use a nova UI) e garanta que o Token seja o mesmo que você definiu no `RELAY_TOKEN` do Coolify.

---

### Por que usar o Coolify?
- **Auto-healing:** Se o hub cair, o Coolify sobe ele de volta.
- **SSL Automático:** Essencial para conexões seguras entre as pontas.
- **Logs:** Você pode ver quem está se conectando ao hub diretamente pelo painel do Coolify.
