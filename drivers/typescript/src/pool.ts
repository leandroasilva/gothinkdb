import { Connection, connect } from './connection';
import { PoolOptions, PoolStats } from './types';

interface PoolConn {
  conn: Connection;
  createdAt: number;
  inUse: boolean;
}

/**
 * Connection pool for GoThinkDB
 */
export class Pool {
  private options: Required<PoolOptions>;
  private conns: PoolConn[] = [];
  private closed = false;
  private totalCreated = 0;
  private totalDestroyed = 0;
  private healthInterval: ReturnType<typeof setInterval> | null = null;

  constructor(options: PoolOptions = {}) {
    this.options = {
      host: options.host || 'localhost',
      port: options.port || 28015,
      db: options.db || 'test',
      user: options.user || 'admin',
      password: options.password || '',
      timeout: options.timeout || 30000,
      maxConns: options.maxConns || 10,
      minConns: options.minConns || 2,
      connMaxLifetime: options.connMaxLifetime || 300000,
    };
  }

  /**
   * Initialize the pool with minimum connections
   */
  async init(): Promise<Pool> {
    if (this.options.minConns > this.options.maxConns) {
      throw new Error(`minConns (${this.options.minConns}) cannot exceed maxConns (${this.options.maxConns})`);
    }

    for (let i = 0; i < this.options.minConns; i++) {
      const pc = await this.createConn();
      this.conns.push(pc);
    }

    // Start health check
    this.healthInterval = setInterval(() => this.removeStaleConns(), 30000);

    return this;
  }

  private async createConn(): Promise<PoolConn> {
    const conn = await connect({
      host: this.options.host,
      port: this.options.port,
      db: this.options.db,
      user: this.options.user,
      password: this.options.password,
      timeout: this.options.timeout,
    });
    this.totalCreated++;
    return { conn, createdAt: Date.now(), inUse: false };
  }

  /**
   * Acquire a connection from the pool
   */
  async acquire(): Promise<Connection> {
    if (this.closed) throw new Error('Pool is closed');

    // Find idle connection
    for (const pc of this.conns) {
      if (!pc.inUse && pc.conn.isOpen()) {
        pc.inUse = true;
        return pc.conn;
      }
    }

    // Create new if under limit
    if (this.conns.length < this.options.maxConns) {
      const pc = await this.createConn();
      pc.inUse = true;
      this.conns.push(pc);
      return pc.conn;
    }

    throw new Error(`Connection pool exhausted (max=${this.options.maxConns})`);
  }

  /**
   * Release a connection back to the pool
   */
  release(conn: Connection): void {
    for (const pc of this.conns) {
      if (pc.conn === conn) {
        pc.inUse = false;
        return;
      }
    }
  }

  /**
   * Execute a function with an auto-released connection
   */
  async exec<T>(fn: (conn: Connection) => Promise<T>): Promise<T> {
    const conn = await this.acquire();
    try {
      return await fn(conn);
    } finally {
      this.release(conn);
    }
  }

  /**
   * Get pool statistics
   */
  stats(): PoolStats {
    let idle = 0, inUse = 0;
    for (const pc of this.conns) {
      if (pc.inUse) inUse++;
      else idle++;
    }
    return {
      totalConns: this.conns.length,
      idleConns: idle,
      inUseConns: inUse,
      maxConns: this.options.maxConns,
      minConns: this.options.minConns,
      totalCreated: this.totalCreated,
      totalDestroyed: this.totalDestroyed,
    };
  }

  private removeStaleConns(): void {
    const now = Date.now();
    const alive: PoolConn[] = [];
    for (const pc of this.conns) {
      if (pc.inUse) {
        alive.push(pc);
        continue;
      }
      if (now - pc.createdAt > this.options.connMaxLifetime || !pc.conn.isOpen()) {
        pc.conn.close();
        this.totalDestroyed++;
        continue;
      }
      alive.push(pc);
    }
    this.conns = alive;
  }

  /**
   * Close all connections in the pool
   */
  async close(): Promise<void> {
    this.closed = true;
    if (this.healthInterval) {
      clearInterval(this.healthInterval);
      this.healthInterval = null;
    }
    for (const pc of this.conns) {
      await pc.conn.close();
      this.totalDestroyed++;
    }
    this.conns = [];
  }
}
