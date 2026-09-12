# GoThinkDB Docker Images

## Quick Start

### Standalone (Single Node)

```bash
docker run -d \
  --name gothinkdb \
  -p 28015:28015 \
  -p 8080:8080 \
  -p 29015:29015 \
  -v gothinkdb-data:/data/gothinkdb \
  halklenson/gothinkdb:latest
```

Or with Docker Compose:

```bash
docker compose up -d
```

Access the dashboard at: http://localhost:8080

**Default credentials:** admin / admin

### Cluster (3 Nodes)

```bash
docker compose -f docker-compose.cluster.yml up -d
```

This starts a 3-node cluster:
- Node 1: Dashboard http://localhost:8080, Driver localhost:28015
- Node 2: Dashboard http://localhost:8081, Driver localhost:28016
- Node 3: Dashboard http://localhost:8082, Driver localhost:28017

## Ports

| Port | Description |
|------|-------------|
| 28015 | ReQL Driver Protocol |
| 8080 | HTTP Admin Dashboard & API |
| 29015 | Cluster Communication |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| GOTHINKDB_SERVER_NAME | Server name | auto-generated |

## Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| -data | Data directory | /data/gothinkdb |
| -driver-address | Driver protocol address | :28015 |
| -http-address | HTTP admin address | :8080 |
| -cluster-address | Cluster communication address | :29015 |
| -join | Address of existing cluster node to join | (none) |
| -server-name | Server name | auto-generated |

## Examples

### Join an existing cluster

```bash
docker run -d \
  --name gothinkdb-node2 \
  -p 28016:28015 \
  -p 8081:8080 \
  -p 29016:29015 \
  -v gothinkdb-node2-data:/data/gothinkdb \
  halklenson/gothinkdb:latest \
  -data /data/gothinkdb \
  -driver-address :28015 \
  -http-address :8080 \
  -cluster-address :29015 \
  -join gothinkdb-node1:29015 \
  -server-name gothinkdb-node2
```

### Custom data directory

```bash
docker run -d \
  --name gothinkdb \
  -p 28015:28015 \
  -p 8080:8080 \
  -v /my/data/path:/data/gothinkdb \
  halklenson/gothinkdb:latest
```

## Health Check

```bash
curl http://localhost:8080/api/health
```

## Drivers

Connect using any ReQL-compatible driver:

### TypeScript/JavaScript

```typescript
import { connect } from 'gothinkdb-driver';

const conn = await connect({
  host: 'localhost',
  port: 28015
});
```

### Go

```go
import "github.com/leandroasilva/gothinkdb/drivers/go"

conn, err := gothinkdb.Connect("localhost:28015")
```

### Rust

```rust
use gothinkdb::Connection;

let conn = Connection::connect("localhost:28015").await?;
```

## License

Apache 2.0

## Source Code

https://github.com/leandroasilva/gothinkdb
