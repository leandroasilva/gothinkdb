import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { connect, Connection, R, Pool } from '../src/index';

const HOST = process.env.GOTHINKDB_HOST || 'localhost';
const PORT = parseInt(process.env.GOTHINKDB_PORT || '28015');

describe('GoThinkDB TypeScript Driver Integration', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
  });

  afterAll(async () => {
    if (conn) await conn.close();
  });

  it('should connect successfully', () => {
    expect(conn.isOpen()).toBe(true);
  });

  it('should list databases', async () => {
    const result = await r.dbList().run(conn);
    expect(result).toBeDefined();
    console.log('DBList:', JSON.stringify(result));
  });

  it('should create and drop database', async () => {
    const createResult = await r.dbCreate('ts_test_db').run(conn);
    console.log('DBCreate:', JSON.stringify(createResult));
    expect(createResult).toBeDefined();

    const dropResult = await r.dbDrop('ts_test_db').run(conn);
    console.log('DBDrop:', JSON.stringify(dropResult));
    expect(dropResult).toBeDefined();
  });

  it('should CRUD documents', async () => {
    const table = r.table('ts_crud_test');

    // Insert
    const insertResult = await table.insert({
      id: 'ts1',
      name: 'TypeScript User',
      age: 25,
      city: 'SP',
      active: true,
    }).run(conn);
    console.log('Insert:', JSON.stringify(insertResult));

    // Get
    const getResult = await table.get('ts1').run(conn);
    console.log('Get:', JSON.stringify(getResult));

    // Update
    const updateResult = await table.get('ts1').update({ age: 26 }).run(conn);
    console.log('Update:', JSON.stringify(updateResult));

    // Get after update
    const afterUpdate = await table.get('ts1').run(conn);
    console.log('After update:', JSON.stringify(afterUpdate));

    // Delete
    const deleteResult = await table.get('ts1').delete().run(conn);
    console.log('Delete:', JSON.stringify(deleteResult));
  });

  it('should insert multiple documents', async () => {
    const table = r.table('ts_multi_test');

    for (let i = 0; i < 10; i++) {
      await table.insert({
        id: `ts_multi_${i}`,
        name: `User${i}`,
        age: 20 + i,
        city: ['SP', 'RJ', 'MG'][i % 3],
        active: i % 2 === 0,
      }).run(conn);
    }
    console.log('Inserted 10 documents');

    // Get each one
    for (let i = 0; i < 10; i++) {
      const doc = await table.get(`ts_multi_${i}`).run(conn);
      expect(doc).toBeDefined();
    }
    console.log('Retrieved all 10 documents');

    // Cleanup
    for (let i = 0; i < 10; i++) {
      await table.get(`ts_multi_${i}`).delete().run(conn);
    }
  });

  it('should test filter query', async () => {
    const table = r.table('ts_filter_test');

    for (let i = 0; i < 5; i++) {
      await table.insert({
        id: `ts_f_${i}`,
        name: `User${i}`,
        active: i % 2 === 0,
      }).run(conn);
    }

    const filterResult = await table.filter({ active: true }).run(conn);
    console.log('Filter result:', JSON.stringify(filterResult));

    // Cleanup
    for (let i = 0; i < 5; i++) {
      await table.get(`ts_f_${i}`).delete().run(conn);
    }
  });
});

describe('Connection Pool', () => {
  it('should create pool with min connections', async () => {
    const pool = await new Pool({
      host: HOST,
      port: PORT,
      maxConns: 5,
      minConns: 2,
    }).init();

    const stats = pool.stats();
    console.log('Pool stats:', JSON.stringify(stats));
    expect(stats.totalConns).toBe(2);
    expect(stats.idleConns).toBe(2);

    await pool.close();
  });

  it('should acquire and release connections', async () => {
    const pool = await new Pool({
      host: HOST,
      port: PORT,
      maxConns: 5,
      minConns: 2,
    }).init();

    const conn = await pool.acquire();
    expect(conn.isOpen()).toBe(true);

    let stats = pool.stats();
    expect(stats.inUseConns).toBe(1);

    pool.release(conn);
    stats = pool.stats();
    expect(stats.inUseConns).toBe(0);

    await pool.close();
  });

  it('should handle concurrent queries', async () => {
    const pool = await new Pool({
      host: HOST,
      port: PORT,
      maxConns: 5,
      minConns: 2,
    }).init();

    const r = new R();
    const promises: Promise<void>[] = [];

    for (let i = 0; i < 10; i++) {
      promises.push(
        pool.exec(async (conn) => {
          await r.dbList().run(conn);
        }).catch((err) => {
          console.log(`Concurrent error (expected for pool exhaustion): ${err.message}`);
        })
      );
    }

    await Promise.allSettled(promises);

    const stats = pool.stats();
    console.log('After concurrent:', JSON.stringify(stats));
    expect(stats.inUseConns).toBe(0);

    await pool.close();
  });

  it('should reject when pool exhausted', async () => {
    const pool = await new Pool({
      host: HOST,
      port: PORT,
      maxConns: 2,
      minConns: 1,
    }).init();

    const conn1 = await pool.acquire();
    const conn2 = await pool.acquire();

    try {
      await pool.acquire();
      expect.fail('Should have thrown');
    } catch (err: any) {
      expect(err.message).toContain('exhausted');
      console.log('Pool exhaustion correctly detected');
    }

    pool.release(conn1);
    pool.release(conn2);
    await pool.close();
  });

  it('should reject after close', async () => {
    const pool = await new Pool({
      host: HOST,
      port: PORT,
      maxConns: 3,
      minConns: 2,
    }).init();

    await pool.close();

    try {
      await pool.acquire();
      expect.fail('Should have thrown');
    } catch (err: any) {
      expect(err.message).toContain('closed');
    }
  });
});
