#!/bin/bash

# Script para iniciar o GoThinkDB com dashboard

echo "🚀 Iniciando GoThinkDB com Dashboard..."
echo ""

# Verificar se o binário existe
if [ ! -f "./gothinkdb" ]; then
    echo "📦 Compilando GoThinkDB..."
    docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go build -o gothinkdb ./cmd/gothinkdb
fi

# Criar diretório de dados
mkdir -p /tmp/gothinkdb-data

# Iniciar o servidor
echo "🌐 Servidor iniciando em:"
echo "   - Dashboard: http://localhost:8080"
echo "   - ReQL Port: 28015"
echo "   - Cluster Port: 29015"
echo ""
echo "📊 Abra o dashboard no navegador: http://localhost:8080"
echo ""
echo "Pressione Ctrl+C para parar o servidor"
echo ""

./gothinkdb \
    --data /tmp/gothinkdb-data \
    --http-address :8080 \
    --driver-address :28015 \
    --cluster-address :29015 \
    --server-name gothinkdb-node1
