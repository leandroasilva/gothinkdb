# 🚀 Guia de Teste do Dashboard GoThinkDB

## Como Iniciar o Dashboard

### Opção 1: Usando o script (Recomendado)

```bash
cd /Users/leandroadasilva/workspace/gothinkdb
./start-dashboard.sh
```

### Opção 2: Manualmente

```bash
cd /Users/leandroadasilva/workspace/gothinkdb

# Compilar (se necessário)
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go build -o gothinkdb ./cmd/gothinkdb

# Criar diretório de dados
mkdir -p /tmp/gothinkdb-data

# Iniciar o servidor
./gothinkdb \
    --data /tmp/gothinkdb-data \
    --http-address :8080 \
    --driver-address :28015 \
    --cluster-address :29015 \
    --server-name gothinkdb-node1
```

## Acessando o Dashboard

Abra seu navegador e acesse: **http://localhost:8080**

## Funcionalidades do Dashboard

### 1. **Server Info** 📊
- Versão do servidor
- Nome do servidor
- Status atual

### 2. **Server Stats** 📈
- Total de queries
- Queries por segundo
- Conexões ativas

### 3. **Databases** 🗄️
- Lista todos os databases
- Criar novo database (botão "+ Create Database")
- Deletar database (botão "Drop")

### 4. **Tables** 📋
- Lista todas as tables do database "test"
- Criar nova table (botão "+ Create Table")
- Deletar table (botão "Drop")

### 5. **Cluster Status** 🌐
- Status do cluster (standalone/cluster)
- Número de membros
- Líder atual

### 6. **Cluster Members** 👥
- Lista todos os membros do cluster
- Status de cada membro

### 7. **Query Editor** 🔍
- Editor de queries ReQL
- Execute queries JSON
- Visualize resultados em tempo real

**Exemplo de query:**
```json
[10, "test_table"]
```

## Testando o Dashboard

### Teste 1: Verificar Health Check
```bash
curl http://localhost:8080/api/health
```

Esperado:
```json
{"status":"ok","server":"gothinkdb-node1","version":"0.1.0"}
```

### Teste 2: Criar Database via API
```bash
curl -X POST http://localhost:8080/api/databases \
  -H "Content-Type: application/json" \
  -d '{"name":"mydb"}'
```

### Teste 3: Listar Databases
```bash
curl http://localhost:8080/api/databases
```

### Teste 4: Criar Table via API
```bash
curl -X POST "http://localhost:8080/api/tables/mytable?db=test"
```

### Teste 5: Listar Tables
```bash
curl "http://localhost:8080/api/tables?db=test"
```

### Teste 6: Executar Query
```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query":[10,"test_table"]}'
```

## Interface do Dashboard

O dashboard possui:
- 🎨 Design moderno com gradientes
- 📱 Layout responsivo
- 🔄 Atualização automática a cada 5 segundos
- 🎯 Cards organizados por funcionalidade
- 💬 Mensagens de erro/sucesso
- 🔍 Query editor com syntax highlighting

## Parando o Servidor

Pressione `Ctrl+C` no terminal onde o servidor está rodando.

## Troubleshooting

### Dashboard não carrega
- Verifique se o servidor está rodando
- Verifique se a porta 8080 não está em uso
- Verifique os logs do servidor

### API não responde
- Verifique se o endpoint está correto
- Verifique se o servidor está rodando
- Teste com `curl http://localhost:8080/api/health`

### Queries não executam
- Verifique se a query está em formato JSON válido
- Verifique se a table existe
- Verifique os logs do servidor

## Próximos Passos

Após testar o dashboard, você pode:
1. Criar drivers para JavaScript, Python, Go, Rust
2. Testar com cluster multi-nó
3. Implementar testes de carga
4. Adicionar mais funcionalidades ao dashboard

---

**Divirta-se testando o GoThinkDB!** 🚀
