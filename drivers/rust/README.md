# GoThinkDB Driver for Rust

Rust driver for [GoThinkDB](https://github.com/leandroasilva/gothinkdb) - a RethinkDB-compatible database written in Go.

## Installation

Add to your `Cargo.toml`:

```toml
[dependencies]
gothinkdb = "0.1"
```

Or use cargo:

```bash
cargo add gothinkdb
```

## Quick Start

```rust
use gothinkdb::{Connection, Query};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Connect to GoThinkDB
    let mut conn = Connection::connect("localhost:28015").await?;

    // Create a table
    conn.run(Query::db_create("test")).await?;
    conn.run(Query::table_create("test", "users")).await?;

    // Insert data
    conn.run(Query::table_insert("test", "users", vec![
        serde_json::json!({
            "name": "John Doe",
            "email": "john@example.com"
        })
    ])).await?;

    // Query data
    let results = conn.run(Query::table_get_all("test", "users")).await?;
    println!("Users: {:?}", results);

    Ok(())
}
```

## Features

- **Async/Await** - Built with Tokio for non-blocking I/O
- **ReQL Compatible** - Same query language as RethinkDB
- **Type Safe** - Strongly typed query results with Serde
- **Connection Pooling** - Efficient connection management
- **Changefeeds** - Real-time data streaming support

## API Reference

### Connection

```rust
use gothinkdb::Connection;

let conn = Connection::connect("localhost:28015").await?;
```

### Queries

```rust
use gothinkdb::Query;

// Database operations
Query::db_create("mydb")
Query::db_drop("mydb")
Query::db_list()

// Table operations
Query::table_create("mydb", "users")
Query::table_drop("mydb", "users")
Query::table_list("mydb")

// Data operations
Query::table_insert("mydb", "users", data)
Query::table_get("mydb", "users", id)
Query::table_get_all("mydb", "users")
Query::table_filter("mydb", "users", filter)
```

## Examples

### Insert and Query

```rust
use gothinkdb::{Connection, Query};
use serde_json::json;

let mut conn = Connection::connect("localhost:28015").await?;

// Insert multiple documents
let data = vec![
    json!({"name": "Alice", "age": 30}),
    json!({"name": "Bob", "age": 25}),
];
conn.run(Query::table_insert("test", "users", data)).await?;

// Get all documents
let users = conn.run(Query::table_get_all("test", "users")).await?;
```

### Changefeeds (Real-time Updates)

```rust
use gothinkdb::{Connection, Query};

let mut conn = Connection::connect("localhost:28015").await?;

// Subscribe to changes
let mut feed = conn.run(Query::table_changes("test", "users")).await?;

while let Some(change) = feed.next().await {
    println!("Change: {:?}", change);
}
```

## Requirements

- Rust 1.70+
- GoThinkDB server running on localhost:28015 (default)

## License

Apache-2.0

## Links

- [GoThinkDB GitHub](https://github.com/leandroasilva/gothinkdb)
- [crates.io](https://crates.io/crates/gothinkdb)
- [docs.rs](https://docs.rs/gothinkdb)
