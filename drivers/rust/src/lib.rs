//! GoThinkDB driver for Rust
//!
//! A RethinkDB-compatible driver for GoThinkDB servers.

pub mod types;
pub mod connection;
pub mod query;
pub mod pool;

// Re-exports
pub use types::*;
pub use connection::{Connection, connect};
pub use query::{R, Query, TableQuery, DbQuery};
pub use pool::{Pool, PoolOptions, PoolStats};
