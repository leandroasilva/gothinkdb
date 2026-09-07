import * as net from 'net';
import { ConnectionOptions, Response, ResponseType } from './types';

const V1_MAGIC = 0x34c2bdc3;
const V0_4_MAGIC = 0x400c2d20;

/**
 * Connection to a GoThinkDB server
 */
export class Connection {
  private socket: net.Socket | null = null;
  private options: Required<ConnectionOptions>;
  private tokenCounter = 0;
  private pendingQueries = new Map<number, {
    resolve: (value: Response) => void;
    reject: (error: Error) => void;
  }>();
  private buffer = Buffer.alloc(0);
  private connected = false;

  constructor(options: ConnectionOptions = {}) {
    this.options = {
      host: options.host || 'localhost',
      port: options.port || 28015,
      db: options.db || 'test',
      user: options.user || 'admin',
      password: options.password || '',
      timeout: options.timeout || 30000,
    };
  }

  /**
   * Connect to the server
   */
  async connect(): Promise<Connection> {
    return new Promise((resolve, reject) => {
      this.socket = new net.Socket();

      this.socket.on('data', (data) => this.handleData(data));
      this.socket.on('error', (err) => {
        this.connected = false;
        reject(err);
      });
      this.socket.on('close', () => {
        this.connected = false;
      });

      this.socket.connect(this.options.port, this.options.host, async () => {
        try {
          await this.handshake();
          this.connected = true;
          resolve(this);
        } catch (err) {
          reject(err);
        }
      });

      setTimeout(() => {
        if (!this.connected) {
          this.socket?.destroy();
          reject(new Error('Connection timeout'));
        }
      }, this.options.timeout);
    });
  }

  /**
   * Perform V1.0 handshake (SCRAM-SHA-256 simplified)
   */
  private async handshake(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Send magic number (V1.0)
      const magic = Buffer.alloc(4);
      magic.writeUInt32LE(V1_MAGIC, 0);
      this.socket!.write(magic);

      // Send auth JSON (null-terminated)
      const auth = JSON.stringify({
        protocol_version: 1,
        authentication_method: 'SCRAM-SHA-256',
        authentication: '',
      }) + '\0';
      this.socket!.write(auth);

      // Wait for response
      const onData = (data: Buffer) => {
        this.buffer = Buffer.concat([this.buffer, data]);
        const nullIdx = this.buffer.indexOf(0);
        if (nullIdx >= 0) {
          const response = this.buffer.subarray(0, nullIdx).toString();
          this.buffer = this.buffer.subarray(nullIdx + 1);
          this.socket!.off('data', onData);

          try {
            const parsed = JSON.parse(response);
            if (parsed.success || parsed.authentication === 'SUCCESS') {
              // Send wire protocol selection
              const protocol = Buffer.alloc(4);
              protocol.writeUInt32LE(0x271ffc41, 0); // JSON protocol
              this.socket!.write(protocol);

              // Wait for SUCCESS response
              const onProtoData = (data: Buffer) => {
                this.buffer = Buffer.concat([this.buffer, data]);
                const idx = this.buffer.indexOf(0);
                if (idx >= 0) {
                  const protoResponse = this.buffer.subarray(0, idx).toString();
                  this.buffer = this.buffer.subarray(idx + 1);
                  this.socket!.off('data', onProtoData);

                  if (protoResponse === 'SUCCESS') {
                    resolve();
                  } else {
                    reject(new Error(`Protocol handshake failed: ${protoResponse}`));
                  }
                }
              };
              this.socket!.on('data', onProtoData);
            } else if (response === 'SUCCESS') {
              resolve();
            } else {
              // Fallback: try legacy handshake
              this.legacyHandshake().then(resolve).catch(reject);
            }
          } catch {
            // If parsing fails, try legacy
            this.legacyHandshake().then(resolve).catch(reject);
          }
        }
      };
      this.socket!.on('data', onData);
    });
  }

  /**
   * Legacy V0.4 handshake
   */
  private async legacyHandshake(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Already connected, just mark as ready
      resolve();
    });
  }

  /**
   * Send a query to the server
   */
  async query<T = unknown>(term: unknown, options?: { db?: string }): Promise<Response<T>> {
    if (!this.connected || !this.socket) {
      throw new Error('Not connected');
    }

    const token = ++this.tokenCounter;
    const query = JSON.stringify([1, term]) + '\0';

    // Send token (8 bytes) + query length (4 bytes) + query
    const tokenBuf = Buffer.alloc(12);
    tokenBuf.writeBigUInt64LE(BigInt(token), 0);
    tokenBuf.writeUInt32LE(Buffer.byteLength(query), 8);

    this.socket.write(Buffer.concat([tokenBuf, Buffer.from(query)]));

    return new Promise((resolve, reject) => {
      this.pendingQueries.set(token, { resolve: resolve as (v: Response) => void, reject });
      setTimeout(() => {
        if (this.pendingQueries.has(token)) {
          this.pendingQueries.delete(token);
          reject(new Error('Query timeout'));
        }
      }, this.options.timeout);
    });
  }

  /**
   * Handle incoming data
   */
  private handleData(data: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, data]);

    while (this.buffer.length >= 12) {
      const token = Number(this.buffer.readBigUInt64LE(0));
      const responseLen = this.buffer.readUInt32LE(8);

      if (this.buffer.length < 12 + responseLen) {
        break; // Need more data
      }

      const responseStr = this.buffer.subarray(12, 12 + responseLen).toString();
      this.buffer = this.buffer.subarray(12 + responseLen);

      try {
        const response = JSON.parse(responseStr) as Response;
        response.token = token;

        const pending = this.pendingQueries.get(token);
        if (pending) {
          this.pendingQueries.delete(token);
          if (response.type === ResponseType.RUNTIME_ERROR ||
              response.type === ResponseType.COMPILE_ERROR ||
              response.type === ResponseType.CLIENT_ERROR) {
            pending.reject(new Error(response.error || 'Query error'));
          } else {
            pending.resolve(response);
          }
        }
      } catch (err) {
        const pending = this.pendingQueries.get(token);
        if (pending) {
          this.pendingQueries.delete(token);
          pending.reject(err instanceof Error ? err : new Error(String(err)));
        }
      }
    }
  }

  /**
   * Close the connection
   */
  async close(): Promise<void> {
    return new Promise((resolve) => {
      if (this.socket) {
        this.socket.end(() => {
          this.connected = false;
          resolve();
        });
      } else {
        resolve();
      }
    });
  }

  /**
   * Check if connected
   */
  isOpen(): boolean {
    return this.connected;
  }

  /**
   * Get the default database
   */
  getDb(): string {
    return this.options.db;
  }
}

/**
 * Connect to a GoThinkDB server
 */
export async function connect(options?: ConnectionOptions): Promise<Connection> {
  const conn = new Connection(options);
  await conn.connect();
  return conn;
}
