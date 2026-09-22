import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { connect, Connection, R, Pool } from '../src/index';
import * as net from 'net';

const HOST = process.env.GOTHINKDB_HOST || 'localhost';
const PORT = parseInt(process.env.GOTHINKDB_PORT || '28015');

/** Check if GoThinkDB server is reachable */
function isServerAvailable(): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = new net.Socket();
    socket.setTimeout(2000);
    socket.on('connect', () => { socket.destroy(); resolve(true); });
    socket.on('timeout', () => { socket.destroy(); resolve(false); });
    socket.on('error', () => { resolve(false); });
    socket.connect(PORT, HOST);
  });
}

let serverAvailable = false;

// ============================================================
// 1. Database Operations
// ============================================================
describe('1. Database Operations', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
  });

  afterAll(async () => {
    if (conn) await conn.close();
  });

  it('should create, list, and drop a database', async () => {
    if (!serverAvailable) return;
    const createResult = await r.dbCreate('ts_comp_test_db').run(conn) as any;
    console.log('DBCreate:', JSON.stringify(createResult));
    expect(createResult.dbs_created).toBe(1);

    const dbList = await r.dbList().run(conn) as string[];
    console.log('DBList:', JSON.stringify(dbList));
    expect(dbList).toContain('ts_comp_test_db');

    const dropResult = await r.dbDrop('ts_comp_test_db').run(conn) as any;
    console.log('DBDrop:', JSON.stringify(dropResult));
    expect(dropResult.dbs_dropped).toBe(1);
  });
});

// ============================================================
// 2. Table Operations
// ============================================================
describe('2. Table Operations', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
  });

  afterAll(async () => {
    if (conn) await conn.close();
  });

  it('should create, list, and drop tables', async () => {
    if (!serverAvailable) return;
    const names = ['ts_users', 'ts_orders', 'ts_products'];
    for (const name of names) {
      try {
        await r.db('test').tableCreate(name).run(conn);
      } catch (e: any) {
        console.log(`TableCreate ${name}: ${e.message}`);
      }
    }

    const tables = await r.db('test').tableList().run(conn) as string[];
    console.log('TableList:', JSON.stringify(tables));
    for (const name of names) {
      expect(tables).toContain(name);
    }

    for (const name of names) {
      try {
        await r.db('test').tableDrop(name).run(conn);
      } catch (e: any) {
        console.log(`TableDrop ${name}: ${e.message}`);
      }
    }
  });
});

// ============================================================
// 3. Insert 1000+ Records
// ============================================================
describe('3. Insert 1000+ Records', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    // Cleanup
    try { await r.table('ts_bulk').delete().run(conn); } catch {}
  });

  afterAll(async () => {
    try { await r.table('ts_bulk').delete().run(conn); } catch {}
    if (conn) await conn.close();
  });

  it('should insert 1100 records and verify count', async () => {
    if (!serverAvailable) return;
    const table = r.table('ts_bulk');
    const cities = ['SP', 'RJ', 'MG', 'PR', 'RS', 'BA', 'CE', 'DF'];
    const start = Date.now();

    for (let i = 0; i < 1100; i++) {
      await table.insert({
        id: `bulk_${String(i).padStart(4, '0')}`,
        name: `User ${i}`,
        email: `user${i}@test.com`,
        age: 18 + Math.floor(Math.random() * 50),
        salary: 3000 + Math.random() * 7000,
        city: cities[Math.floor(Math.random() * cities.length)],
        active: i % 2 === 0,
        score: Math.random() * 100,
      }).run(conn);
    }

    const elapsed = Date.now() - start;
    console.log(`Inserted 1100 records in ${elapsed}ms`);

    const allDocs = await table.getAll().run(conn) as any[];
    console.log(`GetAll returned ${allDocs.length} documents`);
    expect(allDocs.length).toBe(1100);
  });
});

// ============================================================
// 4. Multi-Table CRUD with Relationships
// ============================================================
describe('4. Multi-Table CRUD', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    // Cleanup
    for (const t of ['ts_mt_users', 'ts_mt_orders', 'ts_mt_products']) {
      try { await r.table(t).delete().run(conn); } catch {}
    }
  });

  afterAll(async () => {
    for (const t of ['ts_mt_users', 'ts_mt_orders', 'ts_mt_products']) {
      try { await r.table(t).delete().run(conn); } catch {}
    }
    if (conn) await conn.close();
  });

  it('should CRUD across multiple related tables', async () => {
    if (!serverAvailable) return;
    const users = r.table('ts_mt_users');
    const orders = r.table('ts_mt_orders');
    const products = r.table('ts_mt_products');

    // Insert 50 users
    for (let i = 0; i < 50; i++) {
      await users.insert({
        id: `u${i}`, name: `User${i}`,
        city: ['SP', 'RJ', 'MG'][i % 3], age: 20 + i,
      }).run(conn);
    }

    // Insert 20 products
    for (let i = 0; i < 20; i++) {
      await products.insert({
        id: `p${i}`, name: `Product${i}`,
        price: 10 + i * 5.5, stock: 100 + i * 10,
      }).run(conn);
    }

    // Insert 100 orders
    for (let i = 0; i < 100; i++) {
      await orders.insert({
        id: `o${i}`, user_id: `u${i % 50}`, product_id: `p${i % 20}`,
        quantity: 1 + Math.floor(Math.random() * 10),
        total: 50 + i * 3.25,
        status: ['pending', 'shipped', 'delivered'][i % 3],
      }).run(conn);
    }
    console.log('Inserted 50 users, 20 products, 100 orders');

    // GET user
    const user = await users.get('u0').run(conn) as any;
    expect(user.name).toBe('User0');

    // UPDATE user
    await users.get('u0').update({ age: 99 }).run(conn);
    const updated = await users.get('u0').run(conn) as any;
    expect(updated.age).toBe(99);

    // DELETE single order
    await orders.get('o0').delete().run(conn);
    const deleted = await orders.get('o0').run(conn);
    expect(deleted).toBeNull();

    // Verify counts
    const uDocs = await users.getAll().run(conn) as any[];
    const oDocs = await orders.getAll().run(conn) as any[];
    const pDocs = await products.getAll().run(conn) as any[];
    console.log(`Users=${uDocs.length}, Orders=${oDocs.length}, Products=${pDocs.length}`);
    expect(uDocs.length).toBe(50);
    expect(oDocs.length).toBe(99);
    expect(pDocs.length).toBe(20);
  });
});

// ============================================================
// 5. Filter, OrderBy, Limit, Skip
// ============================================================
describe('5. Filter, OrderBy, Limit, Skip', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    try { await r.table('ts_query').delete().run(conn); } catch {}
    const table = r.table('ts_query');
    for (let i = 0; i < 100; i++) {
      await table.insert({
        id: `q${String(i).padStart(3, '0')}`, name: `Item${i}`,
        city: ['SP', 'RJ', 'MG', 'PR'][i % 4],
        active: i % 2 === 0, score: i, price: 10 + i * 1.5,
      }).run(conn);
    }
  });

  afterAll(async () => {
    try { await r.table('ts_query').delete().run(conn); } catch {}
    if (conn) await conn.close();
  });

  it('should filter by active=true', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query').filter({ active: true }).run(conn) as any[];
    console.log(`Filter active=true: ${result.length} results`);
    expect(result.length).toBe(50);
  });

  it('should filter by city=SP', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query').filter({ city: 'SP' }).run(conn) as any[];
    console.log(`Filter city=SP: ${result.length} results`);
    expect(result.length).toBe(25);
  });

  it('should order by score', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query').orderBy(['score']).run(conn) as any[];
    console.log(`OrderBy score: ${result.length} results`);
    expect(result.length).toBe(100);
    if (result.length > 1) {
      expect(result[0].score).toBeLessThanOrEqual(result[result.length - 1].score);
    }
  });

  it('should limit to 10', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query').limit(10).run(conn) as any[];
    expect(result.length).toBe(10);
  });

  it('should skip 90', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query').skip(90).run(conn) as any[];
    expect(result.length).toBe(10);
  });

  it('should combine filter+orderBy+limit', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_query')
      .filter({ active: true })
      .orderBy(['score'])
      .limit(5)
      .run(conn) as any[];
    console.log(`Filter+OrderBy+Limit: ${result.length} results`);
    expect(result.length).toBeLessThanOrEqual(5);
  });
});

// ============================================================
// 6. Aggregations
// ============================================================
describe('6. Aggregations (Count, Sum, Avg, Min, Max)', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    try { await r.table('ts_agg').delete().run(conn); } catch {}
    const table = r.table('ts_agg');
    for (let i = 0; i < 200; i++) {
      await table.insert({
        id: `a${i}`, value: i + 1,
        group: ['A', 'B', 'C', 'D'][i % 4],
      }).run(conn);
    }
  });

  afterAll(async () => {
    try { await r.table('ts_agg').delete().run(conn); } catch {}
    if (conn) await conn.close();
  });

  it('should count 200', async () => {
    if (!serverAvailable) return;
    const count = await r.table('ts_agg').count().run(conn) as number;
    console.log(`Count: ${count}`);
    expect(count).toBe(200);
  });

  it('should sum values to 20100', async () => {
    if (!serverAvailable) return;
    const sum = await r.table('ts_agg').sum('value').run(conn) as number;
    console.log(`Sum(value): ${sum}`);
    expect(sum).toBe(20100);
  });

  it('should avg values to 100.5', async () => {
    if (!serverAvailable) return;
    const avg = await r.table('ts_agg').avg('value').run(conn) as number;
    console.log(`Avg(value): ${avg}`);
    expect(avg).toBe(100.5);
  });

  it('should find min=1 and max=200', async () => {
    if (!serverAvailable) return;
    const min = await r.table('ts_agg').min('value').run(conn) as unknown as number;
    const max = await r.table('ts_agg').max('value').run(conn) as unknown as number;
    console.log(`Min: ${min}, Max: ${max}`);
    expect(min).toBe(1);
    expect(max).toBe(200);
  });

  it('should group into 4 groups', async () => {
    if (!serverAvailable) return;
    const groups = await r.table('ts_agg').group('group').run(conn) as any[];
    console.log(`Group: ${groups.length} groups`);
    expect(groups.length).toBe(4);
  });
});

// ============================================================
// 7. Index + Between
// ============================================================
describe('7. Index Operations + Between', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    try { await r.table('ts_idx').delete().run(conn); } catch {}
    const table = r.table('ts_idx');
    for (let i = 0; i < 100; i++) {
      await table.insert({
        id: `i${String(i).padStart(3, '0')}`,
        name: `Item${i}`, age: String(18 + (i % 50)),
      }).run(conn);
    }
  });

  afterAll(async () => {
    try { await r.table('ts_idx').delete().run(conn); } catch {}
    if (conn) await conn.close();
  });

  it('should create, list, and drop an index', async () => {
    if (!serverAvailable) return;
    const table = r.table('ts_idx');
    const createResult = await table.indexCreate('age_idx', 'age' as any).run(conn) as any;
    console.log('IndexCreate:', JSON.stringify(createResult));
    expect(createResult.created).toBe(1);

    const indexes = await table.indexList().run(conn) as string[];
    console.log('IndexList:', JSON.stringify(indexes));
    expect(indexes).toContain('age_idx');

    const dropResult = await table.indexDrop('age_idx').run(conn) as any;
    console.log('IndexDrop:', JSON.stringify(dropResult));
    expect(dropResult.dropped).toBe(1);
  });

  it('should query between range', async () => {
    if (!serverAvailable) return;
    const table = r.table('ts_idx');
    await table.indexCreate('age_idx2', 'age' as any).run(conn);
    const result = await table.between('age_idx2', '20', '30').run(conn) as any[];
    console.log(`Between 20-30: ${result.length} results`);
    expect(result.length).toBeGreaterThan(0);
    await table.indexDrop('age_idx2').run(conn);
  });
});

// ============================================================
// 8. Projection (HasFields, Without)
// ============================================================
describe('8. Projection', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    try { await r.table('ts_proj').delete().run(conn); } catch {}
    const table = r.table('ts_proj');
    for (let i = 0; i < 10; i++) {
      const doc: any = { id: `pr${i}`, name: `Name${i}`, email: `email${i}@test.com` };
      if (i % 2 === 0) doc.phone = '111-1111';
      await table.insert(doc).run(conn);
    }
  });

  afterAll(async () => {
    try { await r.table('ts_proj').delete().run(conn); } catch {}
    if (conn) await conn.close();
  });

  it('should filter by hasFields', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_proj').hasFields('phone').run(conn) as any[];
    console.log(`HasFields(phone): ${result.length}`);
    expect(result.length).toBe(5);
  });

  it('should remove fields with without', async () => {
    if (!serverAvailable) return;
    const result = await r.table('ts_proj').without('email').run(conn) as any[];
    console.log(`Without(email): ${result.length}`);
    expect(result.length).toBe(10);
    if (result.length > 0) {
      expect(result[0].email).toBeUndefined();
    }
  });
});

// ============================================================
// 9. Cross-Table Join
// ============================================================
describe('9. Cross-Table Join', () => {
  let conn: Connection;
  let r: R;

  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
    if (!serverAvailable) return;
    conn = await connect({ host: HOST, port: PORT });
    r = new R(conn);
    for (const t of ['ts_ju', 'ts_jo']) {
      try { await r.table(t).delete().run(conn); } catch {}
    }
    for (let i = 0; i < 10; i++) {
      await r.table('ts_ju').insert({
        id: `ju${i}`, user_id: `uid${i}`, name: `User${i}`,
      }).run(conn);
    }
    for (let i = 0; i < 15; i++) {
      await r.table('ts_jo').insert({
        id: `jo${i}`, user_id: `uid${i % 10}`, product: `Prod${i % 5}`,
        amount: 100 + i * 10,
      }).run(conn);
    }
  });

  afterAll(async () => {
    for (const t of ['ts_ju', 'ts_jo']) {
      try { await r.table(t).delete().run(conn); } catch {}
    }
    if (conn) await conn.close();
  });

  it('should inner join', async () => {
    if (!serverAvailable) return;
    const left = r.table('ts_ju');
    const right = r.table('ts_jo');
    const result = await left.innerJoin(right, { match: 'user_id' } as any).run(conn) as any[];
    console.log(`InnerJoin: ${result.length} results`);
    expect(result.length).toBeGreaterThan(0);
  });

  it('should outer join', async () => {
    if (!serverAvailable) return;
    const left = r.table('ts_ju');
    const right = r.table('ts_jo');
    const result = await left.outerJoin(right, { match: 'user_id' } as any).run(conn) as any[];
    console.log(`OuterJoin: ${result.length} results`);
    expect(result.length).toBeGreaterThanOrEqual(10);
  });
});

// ============================================================
// 10. Connection Pool Control
// ============================================================
describe('10. Connection Pool', () => {
  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
  });

  it('should manage pool lifecycle', async () => {
    if (!serverAvailable) return;
    const pool = await new Pool({
      host: HOST, port: PORT, maxConns: 10, minConns: 3,
    }).init();

    let stats = pool.stats();
    console.log('Initial:', JSON.stringify(stats));
    expect(stats.totalConns).toBe(3);

    // Acquire 5
    const conns = [];
    for (let i = 0; i < 5; i++) {
      conns.push(await pool.acquire());
    }
    stats = pool.stats();
    expect(stats.inUseConns).toBe(5);

    // Query on each
    const r = new R();
    for (const c of conns) {
      await r.dbList().run(c);
    }

    // Release all
    for (const c of conns) pool.release(c);
    stats = pool.stats();
    expect(stats.inUseConns).toBe(0);

    // Exhaust pool
    for (let i = 0; i < 10; i++) await pool.acquire();
    try {
      await pool.acquire();
      expect.fail('Should have thrown');
    } catch (e: any) {
      expect(e.message).toContain('exhausted');
    }

    await pool.close();
  });
});

// ============================================================
// 11. Concurrent Pool Operations
// ============================================================
describe('11. Concurrent Pool Ops', () => {
  beforeAll(async () => {
    serverAvailable = await isServerAvailable();
  });

  it('should handle concurrent queries via pool', async () => {
    if (!serverAvailable) return;
    const pool = await new Pool({
      host: HOST, port: PORT, maxConns: 10, minConns: 3,
    }).init();

    const r = new R();
    const table = r.table('ts_concurrent');
    try { await pool.exec(async (c) => { await r.table('ts_concurrent').delete().run(c); }); } catch {}

    // Insert 100 records
    for (let i = 0; i < 100; i++) {
      await pool.exec(async (c) => {
        await table.insert({ id: `c${i}`, value: i }).run(c);
      });
    }

    // 50 concurrent reads
    const start = Date.now();
    const results = await Promise.allSettled(
      Array.from({ length: 50 }, (_, i) =>
        pool.exec(async (c) => {
          return await r.table('ts_concurrent').get(`c${i % 100}`).run(c);
        })
      )
    );

    const errors = results.filter(r => r.status === 'rejected').length;
    console.log(`50 concurrent reads in ${Date.now() - start}ms, errors: ${errors}`);

    const stats = pool.stats();
    console.log('Pool stats:', JSON.stringify(stats));

    await pool.exec(async (c) => { await r.table('ts_concurrent').delete().run(c); });
    await pool.close();
  });
});
