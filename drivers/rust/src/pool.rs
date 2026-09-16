use std::sync::Arc;
use tokio::sync::Mutex;
use std::time::Instant;

use crate::connection::{Connection, connect};
use crate::types::*;

/// Pool configuration.
#[derive(Debug, Clone)]
pub struct PoolOptions {
    pub host: String,
    pub port: u16,
    pub db: String,
    pub user: String,
    pub password: String,
    pub max_conns: usize,
    pub min_conns: usize,
}

impl Default for PoolOptions {
    fn default() -> Self {
        Self {
            host: "localhost".to_string(),
            port: 28015,
            db: "test".to_string(),
            user: "admin".to_string(),
            password: String::new(),
            max_conns: 10,
            min_conns: 2,
        }
    }
}

/// Pool statistics.
#[derive(Debug, Clone)]
pub struct PoolStats {
    pub total_conns: usize,
    pub idle_conns: usize,
    pub in_use_conns: usize,
    pub max_conns: usize,
    pub min_conns: usize,
    pub total_created: u64,
    pub total_destroyed: u64,
}

struct PoolEntry {
    conn: Arc<Connection>,
    in_use: bool,
}

/// Connection pool for GoThinkDB.
pub struct Pool {
    opts: PoolOptions,
    conns: Arc<Mutex<Vec<PoolEntry>>>,
    closed: Arc<Mutex<bool>>,
    total_created: Arc<Mutex<u64>>,
    total_destroyed: Arc<Mutex<u64>>,
}

impl Pool {
    /// Create and initialize the pool.
    pub async fn new(opts: PoolOptions) -> Result<Self> {
        if opts.min_conns > opts.max_conns {
            return Err(Error::Connection("min_conns cannot exceed max_conns".to_string()));
        }

        let pool = Pool {
            opts,
            conns: Arc::new(Mutex::new(Vec::new())),
            closed: Arc::new(Mutex::new(false)),
            total_created: Arc::new(Mutex::new(0)),
            total_destroyed: Arc::new(Mutex::new(0)),
        };

        let mut conns = pool.conns.lock().await;
        for _ in 0..pool.opts.min_conns {
            let c = connect(ConnectOptions {
                host: pool.opts.host.clone(),
                port: pool.opts.port,
                db: pool.opts.db.clone(),
                user: pool.opts.user.clone(),
                password: pool.opts.password.clone(),
            }).await?;
            *pool.total_created.lock().await += 1;
            conns.push(PoolEntry {
                conn: Arc::new(c),
                in_use: false,
            });
        }
        drop(conns);

        Ok(pool)
    }

    /// Execute a function with a pooled connection.
    pub async fn exec<F, Fut, T>(&self, f: F) -> Result<T>
    where
        F: FnOnce(Arc<Connection>) -> Fut,
        Fut: std::future::Future<Output = Result<T>>,
    {
        if *self.closed.lock().await {
            return Err(Error::PoolClosed);
        }

        let mut conns = self.conns.lock().await;

        // Find idle connection
        let mut found_idx = None;
        for (i, entry) in conns.iter().enumerate() {
            if !entry.in_use {
                found_idx = Some(i);
                break;
            }
        }

        let conn_arc = if let Some(idx) = found_idx {
            conns[idx].in_use = true;
            conns[idx].conn.clone()
        } else if conns.len() < self.opts.max_conns {
            let c = connect(ConnectOptions {
                host: self.opts.host.clone(),
                port: self.opts.port,
                db: self.opts.db.clone(),
                user: self.opts.user.clone(),
                password: self.opts.password.clone(),
            }).await?;
            *self.total_created.lock().await += 1;
            let arc = Arc::new(c);
            conns.push(PoolEntry { conn: arc.clone(), in_use: true });
            arc
        } else {
            return Err(Error::PoolExhausted);
        };

        drop(conns);

        let result = f(conn_arc.clone()).await;

        // Release
        let mut conns = self.conns.lock().await;
        for entry in conns.iter_mut() {
            if Arc::ptr_eq(&entry.conn, &conn_arc) {
                entry.in_use = false;
                break;
            }
        }

        result
    }

    /// Get pool statistics.
    pub async fn stats(&self) -> PoolStats {
        let conns = self.conns.lock().await;
        let mut idle = 0usize;
        let mut in_use = 0usize;
        for entry in conns.iter() {
            if entry.in_use { in_use += 1; } else { idle += 1; }
        }
        PoolStats {
            total_conns: conns.len(),
            idle_conns: idle,
            in_use_conns: in_use,
            max_conns: self.opts.max_conns,
            min_conns: self.opts.min_conns,
            total_created: *self.total_created.lock().await,
            total_destroyed: *self.total_destroyed.lock().await,
        }
    }

    /// Close all connections.
    pub async fn close(&self) -> Result<()> {
        *self.closed.lock().await = true;
        let mut conns = self.conns.lock().await;
        for entry in conns.drain(..) {
            let _ = entry.conn.close().await;
            *self.total_destroyed.lock().await += 1;
        }
        Ok(())
    }
}
