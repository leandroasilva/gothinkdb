//! GoThinkDB driver for Rust
//!
//! A RethinkDB-compatible driver for GoThinkDB servers.
//!
//! # Example
//! ```no_run
//! use gothinkdb::{Connection, ConnectOptions, R};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let conn = Connection::connect(ConnectOptions::default()).await?;
//!     let r = R::new();
//!
//!     // Insert a document
//!     r.table("users").insert(serde_json::json!({"name": "Alice"})).run(&conn).await?;
//!
//!     // Query documents
//!     let results = r.table("users")
//!         .into_query()
//!         .filter(serde_json::json!({"active": true}))
//!         .run(&conn).await?;
//!
//!     conn.close().await?;
//!     Ok(())
//! }
//! ```

pub mod types;
pub mod connection;
pub mod query;

// Re-exports
pub use types::*;
pub use connection::{Connection, connect};
pub use query::{R, Query, TableQuery, DbQuery};
