/**
 * GoThinkDB Driver Types
 */

export interface ConnectionOptions {
  host?: string;
  port?: number;
  db?: string;
  user?: string;
  password?: string;
  timeout?: number;
}

export interface QueryOptions {
  db?: string;
  binaryFormat?: 'raw' | 'native';
  timeFormat?: 'raw' | 'native';
}

export interface Response<T = unknown> {
  type: ResponseType;
  data: T;
  token: number;
  notes?: string[];
  profile?: unknown;
  error?: string;
}

export enum ResponseType {
  SUCCESS_ATOM = 1,
  SUCCESS_SEQUENCE = 2,
  SUCCESS_PARTIAL = 3,
  WAIT_COMPLETE = 4,
  CLIENT_ERROR = 16,
  COMPILE_ERROR = 17,
  RUNTIME_ERROR = 18,
}

export interface Datum {
  type?: number;
  value?: unknown;
}

export interface ChangeEvent<T = Record<string, unknown>> {
  new_val: T | null;
  old_val: T | null;
}

export type QueryTerm = [number, ...unknown[]];

export interface PoolOptions extends ConnectionOptions {
  maxConns?: number;
  minConns?: number;
  connMaxLifetime?: number;
}

export interface PoolStats {
  totalConns: number;
  idleConns: number;
  inUseConns: number;
  maxConns: number;
  minConns: number;
  totalCreated: number;
  totalDestroyed: number;
}
