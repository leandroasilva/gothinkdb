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

```javascript
const r = require('rethinkdb');

const conn = await r.connect({host: 'localhost', port: 28015});

// Create a table
await r.db('test').tableCreate('users').run(conn);

// Insert data
await r.table('users').insert({
  id: 1,
  name: 'John Doe',
  email: 'john@example.com'
}).run(conn);

// Query data
const cursor = await r.table('users').filter({name: 'John Doe'}).run(conn);
const users = await cursor.toArray();
console.log(users);

// Real-time changefeed
const feed = await r.table('users').changes().run(conn);
feed.each((err, change) => {
  console.log('Change:', change);
});
```

### Go

```go
package main

import (
    "fmt"
    "github.com/leandroasilva/gothinkdb/drivers/go"
)

func main() {
    conn, err := gothinkdb.Connect("localhost:28015")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Create table
    err = gothinkdb.TableCreate(conn, "test", "users")
    
    // Insert
    err = gothinkdb.Insert(conn, "test.users", map[string]interface{}{
        "id": 1,
        "name": "John Doe",
    })
    
    // Query
    results, err := gothinkdb.Table(conn, "test.users").
        Filter(map[string]interface{}{"name": "John Doe"}).
        Run()
}
```

## Development

### Prerequisites

- Docker and Docker Compose
- Go 1.23+ (optional, for local development)
- Make

### Common Commands

```bash
# Show all available commands
make help

# Build Docker image
make docker-build

# Run tests in Docker
make test-docker

# Clean up everything
make clean
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

## Architecture

GoThinkDB is built with a clean, modular architecture:

```
gothinkdb/
├── cmd/gothinkdb/        # Main binary
├── internal/
│   ├── protocol/          # Wire protocol (ReQL compatible)
│   ├── query/             # Query parser and evaluator
│   ├── storage/           # Storage engine (B-tree, page cache)
│   ├── cluster/           # Clustering, Raft, replication
│   ├── admin/             # HTTP admin API
│   ├── rpc/               # Inter-node communication
│   └── config/            # Configuration
├── pkg/
│   └── datum/             # ReQL data types
├── drivers/               # Client drivers
│   ├── go/                # Go driver
│   ├── js/                # JavaScript/TypeScript driver
│   └── rs/                # Rust driver
└── dashboard/             # Web dashboard (shadcn-ui)
```

## Project Status

This project is under active development following a phased approach:

- [x] **Phase 0**: Infrastructure & Docker setup
- [ ] **Phase 1**: Data types (Datum)
- [ ] **Phase 2**: Wire protocol
- [ ] **Phase 3**: Storage engine
- [ ] **Phase 4**: B-Tree & CRUD
- [ ] **Phase 5**: ReQL core
- [ ] **Phase 6**: Secondary indexes & changefeeds
- [ ] **Phase 7**: Admin API & system tables
- [ ] **Phase 8**: RPC & clustering
- [ ] **Phase 9**: Raft consensus
- [ ] **Phase 10**: Consistency & replication
- [ ] **Phase 11**: Advanced features (geo, JS, etc)
- [ ] **Phase 12**: Dashboard & CLI tools

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
