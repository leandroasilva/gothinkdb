# GoThinkDB

**RethinkDB compatible database written in Go**

GoThinkDB is a complete reimplementation of RethinkDB in Go, providing full API compatibility while leveraging Go's simplicity, performance, and excellent concurrency model.

## Features

- **Full ReQL Compatibility** - Same query language and API as RethinkDB
- **Real-time Updates** - Changefeeds for live data streams
- **Distributed Architecture** - Automatic sharding and replication
- **Easy Administration** - Modern web dashboard and intuitive API
- **High Performance** - Built with Go's efficient concurrency primitives
- **Production Ready** - Comprehensive test coverage and Docker support

## Quick Start

### Using Docker (Recommended)

```bash
# Start single node
make docker-up

# Or start 3-node cluster
make docker-cluster-up

# View logs
make docker-logs

# Stop
make docker-down
```

### Access Points

- **Driver Protocol**: `localhost:28015` (ReQL connections)
- **HTTP Admin**: `http://localhost:8080` (Web dashboard)
- **Health Check**: `http://localhost:8080/api/health`

### Cluster Mode

```bash
# Start 3-node cluster
make docker-cluster-up

# Nodes available at:
# Node 1: localhost:28015 (driver), localhost:8080 (HTTP)
# Node 2: localhost:28016 (driver), localhost:8081 (HTTP)
# Node 3: localhost:28017 (driver), localhost:8082 (HTTP)
```

## Using the Database

### JavaScript/TypeScript

```bash
npm install gothinkdb-driver
```

```typescript
import { connect, r } from 'gothinkdb-driver';

const conn = await connect({ host: 'localhost', port: 28015 });

// Create a table
await r.db('test').tableCreate('users').run(conn);

// Insert data
await r.table('users').insert({
  name: 'John Doe',
  email: 'john@example.com'
}).run(conn);

// Query data
const users = await r.table('users')
  .filter({ name: 'John Doe' })
  .run(conn);

// Real-time changefeed
const feed = await r.table('users').changes().run(conn);
```

### Go

```bash
go get github.com/leandroasilva/gothinkdb/drivers/go
```

```go
package main

import (
    "fmt"
    gothinkdb "github.com/leandroasilva/gothinkdb/drivers/go"
)

func main() {
    conn, err := gothinkdb.Connect(gothinkdb.ConnectOptions{
        Host: "localhost",
        Port: 28015,
    })
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    r := gothinkdb.NewRWithConn(conn)

    // Create table
    r.DB("test").TableCreate("users").Run(conn)

    // Insert data
    r.Table("users").Insert(map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
    }).Run(conn)

    // Query data
    results, _ := r.Table("users").
        Filter(map[string]interface{}{"name": "John Doe"}).
        Run(conn)
    fmt.Println(string(results))
}
```

### Rust

```bash
# Cargo.toml
# gothinkdb = "0.1"
```

```rust
use gothinkdb::{Connection, ConnectOptions, R};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let conn = Connection::connect(ConnectOptions::default()).await?;
    let r = R::new();

    // Insert a document
    r.table("users")
        .insert(serde_json::json!({"name": "Alice"}))
        .run(&conn).await?;

    // Query documents
    let results = r.table("users")
        .into_query()
        .filter(serde_json::json!({"active": true}))
        .run(&conn).await?;

    println!("Results: {:?}", results);
    conn.close().await?;
    Ok(())
}
```

## Development

### Prerequisites

- Docker and Docker Compose
- Go 1.23+ (for building the server)
- Node.js 20+ (for dashboard development)
- Rust (optional, for Rust driver development)
- Make

### Common Commands

```bash
# Show all available commands
make help

# Docker
make docker-build          # Build Docker image (multi-stage)
make docker-up             # Start single node
make docker-cluster-up     # Start 3-node cluster
make docker-down           # Stop all containers
make docker-logs           # View logs

# Dashboard
make dashboard-install     # Install dashboard dependencies
make dashboard-dev         # Run dashboard dev server
make dashboard-build       # Build dashboard for production

# Drivers
make driver-ts-test        # Test TypeScript driver
make driver-go-test        # Test Go driver
make driver-rust-test      # Test Rust driver

# Testing
make test                  # Run all Go tests
make test-docker           # Run tests in Docker

# Build
make build                 # Build Go binary
make clean                 # Clean build artifacts
```

### Local Development (with Go installed)

```bash
# Initialize module
make init

# Download dependencies
make deps

# Run tests
make test

# Build binary
make build

# Run with hot-reload (requires air)
make dev
```

## Dashboard

GoThinkDB includes a full-featured web dashboard built with React, TypeScript, Tailwind CSS, and shadcn/ui.

### Pages

- **Dashboard** - Cluster overview with status cards, performance charts (queries/sec, latency), server statistics, and alerts
- **Tables** - Browse databases and tables, view document counts, create/drop tables, manage indexes
- **Servers** - Monitor connected servers with status, uptime, and resource usage
- **Data Explorer** - Interactive ReQL query editor with syntax highlighting, query history, and results in JSON/table view
- **Logs** - Real-time log viewer with severity and server filters

### Real-time Updates

The dashboard connects via WebSocket for live updates on server stats, table changes, and log entries.

### Development

```bash
# Install dashboard dependencies
cd dashboard && npm install

# Run dashboard dev server (proxies API to localhost:8080)
npm run dev

# Build dashboard for production
npm run build
```

The dashboard build output (`dashboard/dist/`) is embedded into the Go binary via `go:embed`.

## Architecture

GoThinkDB is built with a clean, modular architecture:

```
gothinkdb/
├── cmd/gothinkdb/           # Main binary (embeds dashboard)
├── internal/
│   ├── protocol/            # Wire protocol (ReQL compatible)
│   ├── query/               # Query parser and evaluator (ReQL)
│   ├── storage/             # Storage engine (B-tree, page cache)
│   ├── cluster/             # Clustering, Raft, replication
│   ├── api/                 # HTTP admin API + WebSocket
│   ├── rpc/                 # Inter-node communication
│   └── config/              # Configuration
├── pkg/
│   └── datum/               # ReQL data types
├── dashboard/               # Web dashboard (React + shadcn/ui)
│   ├── src/
│   │   ├── pages/           # Dashboard, Tables, Servers, Explorer, Logs
│   │   ├── components/      # UI components (shadcn/ui)
│   │   └── hooks/           # React hooks (useRealtime)
│   └── dist/                # Build output (embedded in Go binary)
├── drivers/
│   ├── typescript/          # TypeScript/JavaScript driver (npm: gothinkdb-driver)
│   ├── go/                  # Go driver (module: github.com/leandroasilva/gothinkdb/drivers/go)
│   └── rust/                # Rust driver (crate: gothinkdb)
├── docker/
│   ├── Dockerfile.single    # Single-node Docker
│   └── docker-compose.*     # Cluster configurations
├── Dockerfile               # Multi-stage build (Node.js + Go + Alpine)
└── Makefile                 # Build, test, and deployment targets
```

## Project Status

All core phases are complete:

- [x] **Phase 0**: Infrastructure & Docker setup
- [x] **Phase 1**: Data types (Datum)
- [x] **Phase 2**: Wire protocol
- [x] **Phase 3**: Storage engine
- [x] **Phase 4**: B-Tree & CRUD
- [x] **Phase 5**: ReQL core
- [x] **Phase 6**: Secondary indexes & changefeeds
- [x] **Phase 7**: Admin API & system tables
- [x] **Phase 8**: RPC & clustering
- [x] **Phase 9**: Raft consensus
- [x] **Phase 10**: Consistency & replication
- [x] **Phase 11**: Advanced features (geo, JS, etc)
- [x] **Phase 12**: Dashboard, drivers & CLI tools

## Contributing

This is a large-scale migration project. Contributions are welcome!

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Copyright (c) 2024 GoThinkDB Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Acknowledgments

- Original [RethinkDB](https://github.com/rethinkdb/rethinkdb) project and contributors
- The Go community for excellent tooling and libraries
