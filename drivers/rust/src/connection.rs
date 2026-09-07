use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::sync::Mutex;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use crate::types::*;

const V1_MAGIC: u32 = 0x34c2bdc3;
const JSON_PROTO: u32 = 0x271ffc41;

/// A connection to a GoThinkDB server.
pub struct Connection {
    stream: Arc<Mutex<TcpStream>>,
    opts: ConnectOptions,
    token: Arc<Mutex<u64>>,
    pending: Arc<Mutex<HashMap<u64, tokio::sync::oneshot::Sender<Response>>>>,
    closed: Arc<Mutex<bool>>,
}

impl Connection {
    /// Connect to a GoThinkDB server.
    pub async fn connect(opts: ConnectOptions) -> Result<Self> {
        let addr = format!("{}:{}", opts.host, opts.port);
        let stream = tokio::time::timeout(
            Duration::from_secs(30),
            TcpStream::connect(&addr),
        )
        .await
        .map_err(|_| Error::Timeout)?
        .map_err(|e| Error::Connection(e.to_string()))?;

        let conn = Connection {
            stream: Arc::new(Mutex::new(stream)),
            opts,
            token: Arc::new(Mutex::new(0)),
            pending: Arc::new(Mutex::new(HashMap::new())),
            closed: Arc::new(Mutex::new(false)),
        };

        conn.handshake().await?;

        // Start read loop
        let stream = conn.stream.clone();
        let pending = conn.pending.clone();
        let closed = conn.closed.clone();
        tokio::spawn(async move {
            Self::read_loop(stream, pending, closed).await;
        });

        Ok(conn)
    }

    /// Perform V1.0 handshake.
    async fn handshake(&self) -> Result<()> {
        let mut stream = self.stream.lock().await;

        // Send magic
        stream.write_all(&V1_MAGIC.to_le_bytes()).await?;

        // Send auth JSON (null-terminated)
        let auth = serde_json::json!({
            "protocol_version": 1,
            "authentication_method": "SCRAM-SHA-256",
            "authentication": ""
        });
        let mut auth_bytes = serde_json::to_vec(&auth)?;
        auth_bytes.push(0);
        stream.write_all(&auth_bytes).await?;

        // Read response (null-terminated)
        let resp = Self::read_null_terminated(&mut *stream).await?;
        let resp_str = String::from_utf8_lossy(&resp);

        // Try parsing as JSON
        if let Ok(parsed) = serde_json::from_slice::<serde_json::Value>(&resp) {
            if parsed.get("success").and_then(|v| v.as_bool()).unwrap_or(false)
                || parsed.get("authentication").and_then(|v| v.as_str()) == Some("SUCCESS")
            {
                // Send protocol selection
                stream.write_all(&JSON_PROTO.to_le_bytes()).await?;

                // Read protocol response
                let proto_resp = Self::read_null_terminated(&mut *stream).await?;
                let proto_str = String::from_utf8_lossy(&proto_resp);
                if proto_str == "SUCCESS" {
                    return Ok(());
                }
                return Err(Error::Handshake(format!("protocol error: {}", proto_str)));
            }
        }

        if resp_str == "SUCCESS" {
            return Ok(());
        }

        Err(Error::Handshake(format!("auth failed: {}", resp_str)))
    }

    /// Read null-terminated bytes from stream.
    async fn read_null_terminated(stream: &mut TcpStream) -> Result<Vec<u8>> {
        let mut buf = Vec::new();
        loop {
            let mut byte = [0u8; 1];
            stream.read_exact(&mut byte).await?;
            if byte[0] == 0 {
                return Ok(buf);
            }
            buf.push(byte[0]);
        }
    }

    /// Read loop that dispatches responses to pending queries.
    async fn read_loop(
        stream: Arc<Mutex<TcpStream>>,
        pending: Arc<Mutex<HashMap<u64, tokio::sync::oneshot::Sender<Response>>>>,
        closed: Arc<Mutex<bool>>,
    ) {
        let mut stream = stream.lock().await;
        loop {
            if *closed.lock().await {
                break;
            }

            // Read header: token (8 bytes) + length (4 bytes)
            let mut header = [0u8; 12];
            if stream.read_exact(&mut header).await.is_err() {
                break;
            }

            let token = u64::from_le_bytes(header[0..8].try_into().unwrap());
            let resp_len = u32::from_le_bytes(header[8..12].try_into().unwrap()) as usize;

            // Read response body
            let mut body = vec![0u8; resp_len];
            if stream.read_exact(&mut body).await.is_err() {
                break;
            }

            // Parse response
            if let Ok(resp) = serde_json::from_slice::<Response>(&body) {
                let mut pending = pending.lock().await;
                if let Some(sender) = pending.remove(&token) {
                    let _ = sender.send(resp);
                }
            }
        }
    }

    /// Send a query to the server.
    pub async fn query(&self, term: serde_json::Value) -> Result<Response> {
        if *self.closed.lock().await {
            return Err(Error::NotConnected);
        }

        let mut token_counter = self.token.lock().await;
        *token_counter += 1;
        let token = *token_counter;
        drop(token_counter);

        let (tx, rx) = tokio::sync::oneshot::channel();
        self.pending.lock().await.insert(token, tx);

        // Build query: [1, term] (START = 1)
        let query = serde_json::json!([1, term]);
        let mut query_bytes = serde_json::to_vec(&query)?;
        query_bytes.push(0); // null-terminate

        // Send: token (8 bytes) + length (4 bytes) + query
        let mut header = Vec::with_capacity(12);
        header.extend_from_slice(&token.to_le_bytes());
        header.extend_from_slice(&(query_bytes.len() as u32).to_le_bytes());

        let mut stream = self.stream.lock().await;
        stream.write_all(&header).await?;
        stream.write_all(&query_bytes).await?;
        drop(stream);

        // Wait for response with timeout
        match tokio::time::timeout(Duration::from_secs(30), rx).await {
            Ok(Ok(resp)) => {
                let rt = ResponseType::from(resp.response_type);
                if rt == ResponseType::RuntimeError
                    || rt == ResponseType::CompileError
                    || rt == ResponseType::ClientError
                {
                    return Err(Error::Query(resp.error.unwrap_or_else(|| "unknown error".to_string())));
                }
                Ok(resp)
            }
            Ok(Err(_)) => Err(Error::Connection("channel closed".to_string())),
            Err(_) => {
                self.pending.lock().await.remove(&token);
                Err(Error::Timeout)
            }
        }
    }

    /// Close the connection.
    pub async fn close(&self) -> Result<()> {
        *self.closed.lock().await = true;
        let mut stream = self.stream.lock().await;
        stream.shutdown().await?;
        Ok(())
    }

    /// Check if the connection is open.
    pub async fn is_open(&self) -> bool {
        !*self.closed.lock().await
    }

    /// Get the default database name.
    pub fn db(&self) -> &str {
        &self.opts.db
    }
}

/// Connect to a GoThinkDB server with default options.
pub async fn connect(opts: ConnectOptions) -> Result<Connection> {
    Connection::connect(opts).await
}
