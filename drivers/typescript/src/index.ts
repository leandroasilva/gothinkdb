import { Connection } from './connection';
import { R } from './query';

export { Connection, connect } from './connection';
export { R, QueryBuilder, TableQuery, DbQuery } from './query';
export {
  ConnectionOptions,
  QueryOptions,
  Response,
  ResponseType,
  Datum,
  ChangeEvent,
  QueryTerm,
} from './types';

/**
 * Create a new R object bound to a connection
 *
 * Usage:
 * ```typescript
 * import { connect, r } from 'gothinkdb-driver';
 *
 * const conn = await connect({ host: 'localhost', port: 28015 });
 * const result = await r.table('users').filter({ active: true }).run(conn);
 * ```
 */
export function createR(conn?: Connection): R {
  return new R(conn);
}

// Default r instance (unbound - requires connection in run())
export const r = new R();
