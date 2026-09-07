import { describe, it, expect } from 'vitest';
import { R, QueryBuilder, TableQuery, DbQuery } from '../src/query';

describe('QueryBuilder', () => {
  const r = new R();

  it('should create a table query', () => {
    const query = r.table('users');
    expect(query).toBeInstanceOf(TableQuery);
    const term = query.toTerm();
    expect(term).toEqual([15, [14, 'test'], 'users']);
  });

  it('should create a db query', () => {
    const query = r.db('mydb');
    expect(query).toBeInstanceOf(DbQuery);
    const term = query.toTerm();
    expect(term).toEqual([14, 'mydb']);
  });

  it('should chain filter', () => {
    const query = r.table('users').filter({ active: true });
    const term = query.toTerm();
    expect(term[0]).toBe(39); // FILTER
  });

  it('should chain limit', () => {
    const query = r.table('users').limit(10);
    const term = query.toTerm();
    expect(term[0]).toBe(42); // LIMIT
    expect(term[2]).toBe(10);
  });

  it('should chain skip', () => {
    const query = r.table('users').skip(5);
    const term = query.toTerm();
    expect(term[0]).toBe(43); // SKIP
    expect(term[2]).toBe(5);
  });

  it('should chain orderBy', () => {
    const query = r.table('users').orderBy('name');
    const term = query.toTerm();
    expect(term[0]).toBe(41); // ORDER_BY
  });

  it('should chain count', () => {
    const query = r.table('users').count();
    const term = query.toTerm();
    expect(term[0]).toBe(86); // COUNT
  });

  it('should chain get', () => {
    const query = r.table('users').get('abc123');
    const term = query.toTerm();
    expect(term[0]).toBe(16); // GET
    expect(term[2]).toBe('abc123');
  });

  it('should chain insert', () => {
    const query = r.table('users').insert({ name: 'John' });
    const term = query.toTerm();
    expect(term[0]).toBe(17); // INSERT
  });

  it('should chain update', () => {
    const query = r.table('users').get('abc').update({ name: 'Jane' });
    const term = query.toTerm();
    expect(term[0]).toBe(18); // UPDATE
  });

  it('should chain delete', () => {
    const query = r.table('users').get('abc').delete();
    const term = query.toTerm();
    expect(term[0]).toBe(19); // DELETE
  });

  it('should create dbList query', () => {
    const query = r.dbList();
    const term = query.toTerm();
    expect(term).toEqual([59]);
  });

  it('should create dbCreate query', () => {
    const query = r.dbCreate('mydb');
    const term = query.toTerm();
    expect(term).toEqual([57, 'mydb']);
  });

  it('should create tableCreate query', () => {
    const query = r.db('test').tableCreate('users');
    const term = query.toTerm();
    expect(term[0]).toBe(60); // TABLE_CREATE
  });

  it('should chain changes', () => {
    const query = r.table('users').changes();
    const term = query.toTerm();
    expect(term[0]).toBe(152); // CHANGES
  });

  it('should chain multiple operations', () => {
    const query = r.table('users')
      .filter({ active: true })
      .orderBy('name')
      .skip(10)
      .limit(5);
    const term = query.toTerm();
    // Should be nested: LIMIT(SKIP(ORDER_BY(FILTER(TABLE, ...), ...), ...), ...)
    expect(term[0]).toBe(42); // LIMIT (outermost)
  });
});
