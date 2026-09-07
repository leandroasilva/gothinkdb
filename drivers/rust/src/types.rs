use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Connection options for connecting to a GoThinkDB server.
#[derive(Debug, Clone)]
pub struct ConnectOptions {
    pub host: String,
    pub port: u16,
    pub db: String,
    pub user: String,
    pub password: String,
}

impl Default for ConnectOptions {
    fn default() -> Self {
        Self {
            host: "localhost".to_string(),
            port: 28015,
            db: "test".to_string(),
            user: "admin".to_string(),
            password: String::new(),
        }
    }
}

/// Response types from the server.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum ResponseType {
    SuccessAtom = 1,
    SuccessSequence = 2,
    SuccessPartial = 3,
    WaitComplete = 4,
    ClientError = 16,
    CompileError = 17,
    RuntimeError = 18,
}

impl From<u8> for ResponseType {
    fn from(v: u8) -> Self {
        match v {
            1 => ResponseType::SuccessAtom,
            2 => ResponseType::SuccessSequence,
            3 => ResponseType::SuccessPartial,
            4 => ResponseType::WaitComplete,
            16 => ResponseType::ClientError,
            17 => ResponseType::CompileError,
            18 => ResponseType::RuntimeError,
            _ => ResponseType::ClientError,
        }
    }
}

/// Server response to a query.
#[derive(Debug, Clone, Deserialize)]
pub struct Response {
    #[serde(rename = "t")]
    pub response_type: u8,
    #[serde(rename = "response")]
    pub data: Option<serde_json::Value>,
    #[serde(rename = "e")]
    pub error: Option<String>,
    #[serde(rename = "n")]
    pub notes: Option<Vec<i32>>,
}

/// A changefeed event.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChangeEvent {
    pub new_val: Option<serde_json::Value>,
    pub old_val: Option<serde_json::Value>,
}

/// Result of a write operation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WriteResult {
    #[serde(default)]
    pub inserted: i64,
    #[serde(default)]
    pub updated: i64,
    #[serde(default)]
    pub deleted: i64,
    #[serde(default)]
    pub replaced: i64,
    #[serde(default)]
    pub errors: i64,
}

/// ReQL term type constants.
pub mod term {
    pub const DATUM: u32 = 1;
    pub const MAKE_ARRAY: u32 = 2;
    pub const MAKE_OBJ: u32 = 3;
    pub const VAR: u32 = 10;
    pub const DB: u32 = 14;
    pub const TABLE: u32 = 15;
    pub const GET: u32 = 16;
    pub const INSERT: u32 = 17;
    pub const UPDATE: u32 = 18;
    pub const DELETE: u32 = 19;
    pub const REPLACE: u32 = 20;
    pub const FILTER: u32 = 39;
    pub const MAP: u32 = 40;
    pub const ORDER_BY: u32 = 41;
    pub const LIMIT: u32 = 42;
    pub const SKIP: u32 = 43;
    pub const GET_ALL: u32 = 78;
    pub const DB_CREATE: u32 = 57;
    pub const DB_DROP: u32 = 58;
    pub const DB_LIST: u32 = 59;
    pub const TABLE_CREATE: u32 = 60;
    pub const TABLE_DROP: u32 = 61;
    pub const TABLE_LIST: u32 = 62;
    pub const INDEX_CREATE: u32 = 75;
    pub const INDEX_DROP: u32 = 76;
    pub const INDEX_LIST: u32 = 77;
    pub const CHANGES: u32 = 152;
    pub const COUNT: u32 = 86;
    pub const SUM: u32 = 87;
    pub const AVG: u32 = 88;
    pub const MIN: u32 = 89;
    pub const MAX: u32 = 90;
    pub const GROUP: u32 = 91;
    pub const UNGROUP: u32 = 92;
    pub const REDUCE: u32 = 93;
    pub const HAS_FIELDS: u32 = 33;
    pub const WITHOUT: u32 = 34;
    pub const MERGE: u32 = 36;
    pub const BETWEEN: u32 = 182;
    pub const INNER_JOIN: u32 = 48;
    pub const OUTER_JOIN: u32 = 49;
}

/// Error types for the driver.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("connection error: {0}")]
    Connection(String),
    #[error("handshake failed: {0}")]
    Handshake(String),
    #[error("query error: {0}")]
    Query(String),
    #[error("not connected")]
    NotConnected,
    #[error("timeout")]
    Timeout,
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
}

pub type Result<T> = std::result::Result<T, Error>;
