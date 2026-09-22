import { Connection } from './connection';
import { R } from './query';

export { Connection, connect } from './connection';
export { R, QueryBuilder, TableQuery, DbQuery } from './query';
export { Pool } from './pool';
export {
  ConnectionOptions,
  QueryOptions,
  Response,
  ResponseType,
  Datum,
  ChangeEvent,
  QueryTerm,
  PoolOptions,
  PoolStats,
} from './types';

export function createR(conn?: Connection): R {
  return new R(conn);
}

export const r = new R();
