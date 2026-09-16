import * as net from 'net';
import { ConnectionOptions, Response, ResponseType } from './types';

const V0_4_MAGIC = 0x400c2d20;
const JSON_PROTOCOL = 0x7e6970c7;

/**
 * Connection to a GoThinkDB server
 */
export class Connection {
  private socket: net.Socket | null = null;
  private options: Required<ConnectionOptions>;
  private tokenCounter = 0;
  private buffer = Buffer.alloc(0);
  private connected = false;
  private queryMutex: Promise<void> = Promise.resolve();

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
   * Perform V0.4 handshake (plaintext auth)
   */
  private async handshake(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Send V0.4 magic
      const magic = Buffer.alloc(4);
      magic.writeUInt32LE(V0_4_MAGIC, 0);
      this.socket!.write(magic);

      // Send auth key size (0)
      const authKeySize = Buffer.alloc(4);
      authKeySize.writeUInt32LE(0, 0);
      this.socket!.write(authKeySize);

      // Send JSON wire protocol
      const proto = Buffer.alloc(4);
      proto.writeUInt32LE(JSON_PROTOCOL, 0);
      this.socket!.write(proto);

      // Wait for SUCCESS\0 response
      const onData = (data: Buffer) => {
        this.buffer = Buffer.concat([this.buffer, data]);
        const nullIdx = this.buffer.indexOf(0);
        if (nullIdx >= 0) {
          const response = this.buffer.subarray(0, nullIdx).toString();
          this.buffer = this.buffer.subarray(nullIdx + 1);
          this.socket!.off('data', onData);

          if (response === 'SUCCESS') {
            resolve();
          } else {
            reject(new Error(`Handshake failed: ${response}`));
          }
        }
      };
      this.socket!.on('data', onData);
    });
  }

  /**
   * Send a query to the server (synchronous: send and read response)
   */
  async query<T = unknown>(term: unknown, _options?: { db?: string }): Promise<Response<T>> {
    if (!this.connected || !this.socket) {
      throw new Error('Not connected');
    }

    const token = ++this.tokenCounter;
    const queryObj = JSON.stringify({ token, type: 1, query: term });
    const queryBuf = Buffer.from(queryObj);

    // Serialize queries using mutex (one at a time)
    const prev = this.queryMutex;
    let releaseMutex: () => void;
    this.queryMutex = new Promise<void>((r) => { releaseMutex = r; });

    await prev;

    return new Promise((resolve, reject) => {
      // Send: length (4 bytes LE) + JSON data
      const header = Buffer.alloc(4);
      header.writeUInt32LE(queryBuf.length, 0);

      this.socket!.write(Buffer.concat([header, queryBuf]));

      const timeout = setTimeout(() => {
        releaseMutex!();
        reject(new Error('Query timeout'));
      }, this.options.timeout);

      // Read response: length (4 bytes) + JSON
      const readResponse = (data: Buffer) => {
        this.buffer = Buffer.concat([this.buffer, data]);

        if (this.buffer.length >= 4) {
          const respLen = this.buffer.readUInt32LE(0);
          if (this.buffer.length >= 4 + respLen) {
            const respStr = this.buffer.subarray(4, 4 + respLen).toString();
            this.buffer = this.buffer.subarray(4 + respLen);
            this.socket!.off('data', readResponse);
            clearTimeout(timeout);

            try {
              const resp = JSON.parse(respStr);
              const response: Response<T> = {
                type: resp.t,
                data: resp.r,
                token,
                notes: resp.n,
              };

              if (response.type === ResponseType.RUNTIME_ERROR ||
                  response.type === ResponseType.COMPILE_ERROR ||
                  response.type === ResponseType.CLIENT_ERROR) {
                const errMsg = (Array.isArray(response.notes) && response.notes.length > 0)
                  ? response.notes[0]
                  : (response.error || 'Query error');
                releaseMutex!();
                reject(new Error(errMsg));
              } else {
                releaseMutex!();
                resolve(response);
              }
            } catch (err) {
              releaseMutex!();
              reject(err instanceof Error ? err : new Error(String(err)));
            }
            return;
          }
        }
        // Need more data
      };

      this.socket!.on('data', readResponse);
    });
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
