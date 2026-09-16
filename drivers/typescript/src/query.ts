import { Connection } from './connection';
import { Response } from './types';

// ReQL Term Types (matching GoThinkDB evaluator)
const TermType = {
  DATUM: 1,
  MAKE_ARRAY: 2,
  MAKE_OBJ: 3,
  HAS_FIELDS: 33,
  WITHOUT: 34,
  MERGE: 36,
  TABLE: 10,
  INSERT: 17,
  UPDATE: 18,
  DELETE: 19,
  REPLACE: 20,
  FILTER: 39,
  MAP: 40,
  ORDER_BY: 41,
  LIMIT: 42,
  SKIP: 43,
  INNER_JOIN: 48,
  OUTER_JOIN: 49,
  GET: 70,
  INDEX_CREATE: 75,
  INDEX_DROP: 76,
  INDEX_LIST: 77,
  GET_ALL: 78,
  COUNT: 86,
  SUM: 87,
  AVG: 88,
  MIN: 89,
  MAX: 90,
  GROUP: 91,
  UNGROUP: 92,
  REDUCE: 93,
  DB: 14,
  DB_CREATE: 57,
  DB_DROP: 58,
  DB_LIST: 59,
  TABLE_CREATE: 60,
  TABLE_DROP: 61,
  TABLE_LIST: 62,
  CHANGES: 152,
  BETWEEN: 172,
  PLUCK: 33,
  WITH_FIELDS: 96,
} as const;

/**
 * Query builder for ReQL queries
 */
export class QueryBuilder<T = unknown> {
  protected term: unknown[];
  protected conn?: Connection;

  constructor(term: unknown[], conn?: Connection) {
    this.term = term;
    this.conn = conn;
  }

  /**
   * Execute the query
   */
  async run(conn?: Connection): Promise<T> {
    const connection = conn || this.conn;
    if (!connection) {
      throw new Error('No connection provided. Pass a connection to run() or create queries from a connection.');
    }
    const response: Response<T> = await connection.query(this.term);
    return response.data;
  }

  /**
   * Get the raw term
   */
  toTerm(): unknown[] {
    return this.term;
  }

  // === Transformations ===

  filter(predicate: Record<string, unknown> | ((doc: unknown) => boolean)): QueryBuilder<T> {
    return new QueryBuilder([TermType.FILTER, this.term, predicate], this.conn);
  }

  map(fn: (doc: unknown) => unknown): QueryBuilder<T> {
    return new QueryBuilder([TermType.MAP, this.term, fn], this.conn);
  }

  orderBy(field: string | string[]): QueryBuilder<T> {
    const index = Array.isArray(field) ? field : [field];
    return new QueryBuilder([TermType.ORDER_BY, this.term, index], this.conn);
  }

  limit(n: number): QueryBuilder<T> {
    return new QueryBuilder([TermType.LIMIT, this.term, n], this.conn);
  }

  skip(n: number): QueryBuilder<T> {
    return new QueryBuilder([TermType.SKIP, this.term, n], this.conn);
  }

  between(lower: unknown, upper: unknown): QueryBuilder<T> {
    return new QueryBuilder([TermType.BETWEEN, this.term, lower, upper], this.conn);
  }

  pluck(...fields: string[]): QueryBuilder<T> {
    return new QueryBuilder([TermType.PLUCK, this.term, fields], this.conn);
  }

  without(...fields: string[]): QueryBuilder<T> {
    return new QueryBuilder([TermType.WITHOUT, this.term, fields], this.conn);
  }

  merge(other: Record<string, unknown>): QueryBuilder<T> {
    return new QueryBuilder([TermType.MERGE, this.term, other], this.conn);
  }

  hasFields(...fields: string[]): QueryBuilder<T> {
    return new QueryBuilder([TermType.HAS_FIELDS, this.term, fields], this.conn);
  }

  // === Aggregations ===

  count(): QueryBuilder<number> {
    return new QueryBuilder<number>([TermType.COUNT, this.term], this.conn);
  }

  sum(field?: string): QueryBuilder<number> {
    return new QueryBuilder<number>([TermType.SUM, this.term, field], this.conn);
  }

  avg(field?: string): QueryBuilder<number> {
    return new QueryBuilder<number>([TermType.AVG, this.term, field], this.conn);
  }

  min(field?: string): QueryBuilder<T> {
    return new QueryBuilder([TermType.MIN, this.term, field], this.conn);
  }

  max(field?: string): QueryBuilder<T> {
    return new QueryBuilder([TermType.MAX, this.term, field], this.conn);
  }

  group(field: string): QueryBuilder<T> {
    return new QueryBuilder([TermType.GROUP, this.term, field], this.conn);
  }

  ungroup(): QueryBuilder<T> {
    return new QueryBuilder([TermType.UNGROUP, this.term], this.conn);
  }

  reduce(fn: (a: unknown, b: unknown) => unknown): QueryBuilder<T> {
    return new QueryBuilder([TermType.REDUCE, this.term, fn], this.conn);
  }

  // === Joins ===

  innerJoin(other: QueryBuilder, predicate: (left: unknown, right: unknown) => boolean): QueryBuilder<T> {
    return new QueryBuilder([TermType.INNER_JOIN, this.term, other.toTerm(), predicate], this.conn);
  }

  outerJoin(other: QueryBuilder, predicate: (left: unknown, right: unknown) => boolean): QueryBuilder<T> {
    return new QueryBuilder([TermType.OUTER_JOIN, this.term, other.toTerm(), predicate], this.conn);
  }

  // === Mutations (work on selections) ===

  update(changes: Record<string, unknown>): QueryBuilder<{ replaced: number }> {
    return new QueryBuilder([TermType.UPDATE, this.term, changes], this.conn);
  }

  delete(): QueryBuilder<{ deleted: number }> {
    return new QueryBuilder([TermType.DELETE, this.term], this.conn);
  }

  replace(doc: unknown): QueryBuilder<{ replaced: number }> {
    return new QueryBuilder([TermType.REPLACE, this.term, doc], this.conn);
  }

  // === Changefeeds ===

  changes(): QueryBuilder<T> {
    return new QueryBuilder([TermType.CHANGES, this.term], this.conn);
  }
}

/**
 * Table reference
 */
export class TableQuery<T = Record<string, unknown>> extends QueryBuilder<T[]> {
  constructor(tableName: string, conn?: Connection) {
    super([TermType.TABLE, tableName], conn);
  }

  get(id: string): QueryBuilder<T> {
    return new QueryBuilder<T>([TermType.GET, this.term, id], this.conn);
  }

  getAll(...keys: unknown[]): QueryBuilder<T[]> {
    return new QueryBuilder<T[]>([TermType.GET_ALL, this.term, ...keys], this.conn);
  }

  insert(doc: T | T[]): QueryBuilder<{ inserted: number; errors: number }> {
    return new QueryBuilder([TermType.INSERT, this.term, doc], this.conn);
  }

  update(changes: Record<string, unknown>): QueryBuilder<{ replaced: number }> {
    return new QueryBuilder([TermType.UPDATE, this.term, changes], this.conn);
  }

  delete(): QueryBuilder<{ deleted: number }> {
    return new QueryBuilder([TermType.DELETE, this.term], this.conn);
  }

  replace(doc: T): QueryBuilder<{ replaced: number }> {
    return new QueryBuilder([TermType.REPLACE, this.term, doc], this.conn);
  }

  indexCreate(name: string, indexFn?: (doc: unknown) => unknown): QueryBuilder<{ created: number }> {
    return new QueryBuilder([TermType.INDEX_CREATE, this.term, name, indexFn], this.conn);
  }

  indexDrop(name: string): QueryBuilder<{ dropped: number }> {
    return new QueryBuilder([TermType.INDEX_DROP, this.term, name], this.conn);
  }

  indexList(): QueryBuilder<string[]> {
    return new QueryBuilder<string[]>([TermType.INDEX_LIST, this.term], this.conn);
  }
}

/**
 * Database reference
 */
export class DbQuery extends QueryBuilder {
  constructor(dbName: string, conn?: Connection) {
    super([TermType.DB, dbName], conn);
  }

  table<T = Record<string, unknown>>(tableName: string): TableQuery<T> {
    return new TableQuery<T>(tableName, this.conn);
  }

  tableCreate(tableName: string): QueryBuilder<{ tables_created: number }> {
    return new QueryBuilder([TermType.TABLE_CREATE, this.term, tableName], this.conn);
  }

  tableDrop(tableName: string): QueryBuilder<{ tables_dropped: number }> {
    return new QueryBuilder([TermType.TABLE_DROP, this.term, tableName], this.conn);
  }

  tableList(): QueryBuilder<string[]> {
    return new QueryBuilder<string[]>([TermType.TABLE_LIST, this.term], this.conn);
  }
}

/**
 * Top-level r object (RethinkDB-compatible API)
 */
export class R {
  private conn?: Connection;

  constructor(conn?: Connection) {
    this.conn = conn;
  }

  db(name: string): DbQuery {
    return new DbQuery(name, this.conn);
  }

  table<T = Record<string, unknown>>(tableName: string, _db?: string): TableQuery<T> {
    return new TableQuery<T>(tableName, this.conn);
  }

  dbCreate(name: string): QueryBuilder<{ dbs_created: number }> {
    return new QueryBuilder([TermType.DB_CREATE, name], this.conn);
  }

  dbDrop(name: string): QueryBuilder<{ dbs_dropped: number }> {
    return new QueryBuilder([TermType.DB_DROP, name], this.conn);
  }

  dbList(): QueryBuilder<string[]> {
    return new QueryBuilder<string[]>([TermType.DB_LIST], this.conn);
  }

  expr(value: unknown): QueryBuilder {
    return new QueryBuilder([TermType.DATUM, value], this.conn);
  }
}
