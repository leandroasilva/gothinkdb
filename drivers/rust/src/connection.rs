use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::sync::Mutex;
use std::sync::Arc;
use std::time::Duration;

use crate::types::*;

const V0_4_MAGIC: u32 = 0x400c2d20;
const JSON_PROTOCOL: u32 = 0x7e6970c7;

/// A connection to a GoThinkDB server.
pub struct Connection {
    stream: Arc<Mutex<TcpStream>>,
    opts: ConnectOptions,
    token: Arc<Mutex<u64>>,
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
            closed: Arc::new(Mutex::new(false)),
        };

        conn.handshake().await?;
        Ok(conn)
    }

    /// Perform V0.4 handshake.
    async fn handshake(&self) -> Result<()> {
        let mut stream = self.stream.lock().await;

        // Send V0.4 magic
        stream.write_all(&V0_4_MAGIC.to_le_bytes()).await?;

        // Send auth key size (0)
        stream.write_all(&0u32.to_le_bytes()).await?;

        // Send JSON wire protocol
        stream.write_all(&JSON_PROTOCOL.to_le_bytes()).await?;

        // Read SUCCESS\0 response
        let resp = Self::read_null_terminated(&mut *stream).await?;
        let resp_str = String::from_utf8_lossy(&resp);

        if resp_str == "SUCCESS" {
            Ok(())
        } else {
            Err(Error::Handshake(format!("handshake failed: {}", resp_str)))
        }
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

    /// Send a query to the server (synchronous: send and read response).
    pub async fn query(&self, term: serde_json::Value) -> Result<Response> {
        if *self.closed.lock().await {
            return Err(Error::NotConnected);
        }

        let mut token_counter = self.token.lock().await;
        *token_counter += 1;
        let token = *token_counter;
        drop(token_counter);

        let mut stream = self.stream.lock().await;

        // Build query JSON: {"token": N, "type": 1, "query": term}
        let query_obj = serde_json::json!({
            "token": token,
            "type": 1,
            "query": term
        });
        let query_bytes = serde_json::to_vec(&query_obj)?;

        // Send: length (4 bytes LE) + JSON data
        let len = query_bytes.len() as u32;
        stream.write_all(&len.to_le_bytes()).await?;
        stream.write_all(&query_bytes).await?;
        stream.flush().await?;

        // Read response: length (4 bytes LE) + JSON data
        let mut len_buf = [0u8; 4];
        stream.read_exact(&mut len_buf).await?;
        let resp_len = u32::from_le_bytes(len_buf) as usize;

        if resp_len > 64 * 1024 * 1024 {
            return Err(Error::Connection("response too large".to_string()));
        }

        let mut body = vec![0u8; resp_len];
        stream.read_exact(&mut body).await?;
        drop(stream);

        let resp: Response = serde_json::from_slice(&body)?;

        let rt = ResponseType::from(resp.response_type);
        if rt == ResponseType::RuntimeError
            || rt == ResponseType::CompileError
            || rt == ResponseType::ClientError
        {
            let err_msg = resp.notes.as_ref()
                .and_then(|n| n.first())
                .cloned()
                .or(resp.error.clone())
                .unwrap_or_else(|| "unknown error".to_string());
            return Err(Error::Query(err_msg));
        }

        Ok(resp)
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
