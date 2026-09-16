#![cfg(test)]

use gothinkdb::*;
use serde_json::json;

async fn test_connect() -> Connection {
    Connection::connect(ConnectOptions {
        host: "localhost".to_string(),
        port: 28015,
        ..Default::default()
    })
    .await
    .expect("failed to connect")
}

async fn cleanup(conn: &Connection, table: &str) {
    let r = R::new();
    let _ = r.table(table).delete().run(conn).await;
}

// ============================================================
// 1. Database Operations
// ============================================================
#[tokio::test]
async fn test_db_operations() {
    let conn = test_connect().await;
    let r = R::new();

    let result = r.db_create("rust_test_db").run(&conn).await.unwrap();
    println!("DBCreate: {}", result);
    assert_eq!(result["dbs_created"], 1);

    let result = r.db_list().run(&conn).await.unwrap();
    println!("DBList: {}", result);
    assert!(result.as_array().unwrap().iter().any(|v| v == "rust_test_db"));

    let result = r.db_drop("rust_test_db").run(&conn).await.unwrap();
    println!("DBDrop: {}", result);
    assert_eq!(result["dbs_dropped"], 1);

    conn.close().await.unwrap();
}

// ============================================================
// 2. Table Operations
// ============================================================
#[tokio::test]
async fn test_table_operations() {
    let conn = test_connect().await;
    let r = R::new();

    for name in &["rust_t1", "rust_t2", "rust_t3"] {
        let _ = r.db("test").table_create(name).run(&conn).await;
    }

    let result = r.db("test").table_list().run(&conn).await.unwrap();
    println!("TableList: {}", result);
    let tables = result.as_array().unwrap();
    assert!(tables.iter().any(|v| v == "rust_t1"));

    for name in &["rust_t1", "rust_t2", "rust_t3"] {
        let _ = r.db("test").table_drop(name).run(&conn).await;
    }

    conn.close().await.unwrap();
}

// ============================================================
// 3. Insert 1000+ Records
// ============================================================
#[tokio::test]
async fn test_insert_1000_records() {
    let conn = test_connect().await;
    let r = R::new();
    let table = r.table("rust_bulk");
    cleanup(&conn, "rust_bulk").await;

    let start = std::time::Instant::now();
    for i in 0..1100 {
        let doc = json!({
            "id": format!("bulk_{:04}", i),
            "name": format!("User {}", i),
            "age": 18 + (i % 50),
            "city": ["SP", "RJ", "MG", "PR"][i % 4],
            "active": i % 2 == 0,
            "score": (i as f64) * 1.5,
        });
        table.clone().insert(doc).run(&conn).await.unwrap();
    }
    println!("Inserted 1100 records in {:?}", start.elapsed());

    let result = table.clone().get_all(vec![]).run(&conn).await.unwrap();
    let docs = result.as_array().unwrap();
    println!("GetAll: {} documents", docs.len());
    assert_eq!(docs.len(), 1100);

    cleanup(&conn, "rust_bulk").await;
    conn.close().await.unwrap();
}

// ============================================================
// 4. Multi-Table CRUD
// ============================================================
#[tokio::test]
async fn test_multi_table_crud() {
    let conn = test_connect().await;
    let r = R::new();

    for t in &["rust_users", "rust_orders", "rust_products"] {
        cleanup(&conn, t).await;
    }

    let users = r.table("rust_users");
    let orders = r.table("rust_orders");
    let products = r.table("rust_products");

    // Insert 50 users
    for i in 0..50 {
        users.clone().insert(json!({
            "id": format!("u{}", i), "name": format!("User{}", i),
            "city": ["SP", "RJ", "MG"][i % 3], "age": 20 + i,
        })).run(&conn).await.unwrap();
    }

    // Insert 20 products
    for i in 0..20 {
        products.clone().insert(json!({
            "id": format!("p{}", i), "name": format!("Product{}", i),
            "price": 10.0 + (i as f64) * 5.5, "stock": 100 + i * 10,
        })).run(&conn).await.unwrap();
    }

    // Insert 100 orders
    for i in 0..100 {
        orders.clone().insert(json!({
            "id": format!("o{}", i), "user_id": format!("u{}", i % 50),
            "product_id": format!("p{}", i % 20), "quantity": 1 + (i % 10),
            "total": 50.0 + (i as f64) * 3.25,
            "status": ["pending", "shipped", "delivered"][i % 3],
        })).run(&conn).await.unwrap();
    }
    println!("Inserted 50 users, 20 products, 100 orders");

    // GET
    let user = users.clone().get("u0").run(&conn).await.unwrap();
    assert_eq!(user["name"], "User0");

    // UPDATE
    users.clone().get("u0").update(json!({"age": 99})).run(&conn).await.unwrap();
    let updated = users.clone().get("u0").run(&conn).await.unwrap();
    assert_eq!(updated["age"], 99);

    // DELETE
    orders.clone().get("o0").delete().run(&conn).await.unwrap();
    let deleted = orders.clone().get("o0").run(&conn).await.unwrap();
    assert!(deleted.is_null());

    // Counts
    let u_count = users.clone().count().run(&conn).await.unwrap();
    let o_count = orders.clone().count().run(&conn).await.unwrap();
    let p_count = products.clone().count().run(&conn).await.unwrap();
    println!("Users={}, Orders={}, Products={}", u_count, o_count, p_count);
    assert_eq!(u_count, 50);
    assert_eq!(o_count, 99);
    assert_eq!(p_count, 20);

    for t in &["rust_users", "rust_orders", "rust_products"] {
        cleanup(&conn, t).await;
    }
    conn.close().await.unwrap();
}

// ============================================================
// 5. Filter, OrderBy, Limit, Skip
// ============================================================
#[tokio::test]
async fn test_filter_order_limit_skip() {
    let conn = test_connect().await;
    let r = R::new();
    let table = r.table("rust_query");
    cleanup(&conn, "rust_query").await;

    for i in 0..100 {
        table.clone().insert(json!({
            "id": format!("q{:03}", i), "name": format!("Item{}", i),
            "city": ["SP", "RJ", "MG", "PR"][i % 4],
            "active": i % 2 == 0, "score": i,
        })).run(&conn).await.unwrap();
    }

    // Filter
    let result = table.clone().into_query().filter(json!({"active": true})).run(&conn).await.unwrap();
    let filtered = result.as_array().unwrap();
    println!("Filter active=true: {} results", filtered.len());
    assert_eq!(filtered.len(), 50);

    // OrderBy
    let result = table.clone().into_query().order_by(vec!["score".to_string()]).run(&conn).await.unwrap();
    let ordered = result.as_array().unwrap();
    println!("OrderBy score: {} results", ordered.len());
    assert_eq!(ordered.len(), 100);

    // Limit
    let result = table.clone().into_query().limit(10).run(&conn).await.unwrap();
    assert_eq!(result.as_array().unwrap().len(), 10);

    // Skip
    let result = table.clone().into_query().skip(90).run(&conn).await.unwrap();
    assert_eq!(result.as_array().unwrap().len(), 10);

    // Combined
    let result = table.clone().into_query()
        .filter(json!({"active": true}))
        .order_by(vec!["score".to_string()])
        .limit(5)
        .run(&conn).await.unwrap();
    assert!(result.as_array().unwrap().len() <= 5);

    cleanup(&conn, "rust_query").await;
    conn.close().await.unwrap();
}

// ============================================================
// 6. Aggregations
// ============================================================
#[tokio::test]
async fn test_aggregations() {
    let conn = test_connect().await;
    let r = R::new();
    let table = r.table("rust_agg");
    cleanup(&conn, "rust_agg").await;

    for i in 0..200 {
        table.clone().insert(json!({
            "id": format!("a{}", i), "value": i + 1,
            "group": ["A", "B", "C", "D"][i % 4],
        })).run(&conn).await.unwrap();
    }

    let count = table.clone().count().run(&conn).await.unwrap();
    println!("Count: {}", count);
    assert_eq!(count, 200);

    let sum = table.clone().sum("value").run(&conn).await.unwrap();
    println!("Sum: {}", sum);
    assert_eq!(sum, 20100);

    let avg = table.clone().avg("value").run(&conn).await.unwrap();
    println!("Avg: {}", avg);
    assert_eq!(avg, 100.5);

    let min = table.clone().min("value").run(&conn).await.unwrap();
    let max = table.clone().max("value").run(&conn).await.unwrap();
    println!("Min: {}, Max: {}", min, max);
    assert_eq!(min, 1);
    assert_eq!(max, 200);

    let groups = table.clone().group("group").run(&conn).await.unwrap();
    println!("Groups: {}", groups.as_array().unwrap().len());
    assert_eq!(groups.as_array().unwrap().len(), 4);

    cleanup(&conn, "rust_agg").await;
    conn.close().await.unwrap();
}

// ============================================================
// 7. Pool Control
// ============================================================
#[tokio::test]
async fn test_pool_control() {
    let pool = Pool::new(PoolOptions {
        max_conns: 10,
        min_conns: 3,
        ..Default::default()
    }).await.unwrap();

    let stats = pool.stats().await;
    println!("Initial pool: total={}", stats.total_conns);
    assert_eq!(stats.total_conns, 3);

    // Execute queries via pool
    let r = R::new();
    for i in 0..5 {
        let result = pool.exec(|conn| {
            let r = R::new();
            async move {
                r.db_list().run(&conn).await
            }
        }).await;
        assert!(result.is_ok());
    }

    let stats = pool.stats().await;
    println!("After queries: total={}, created={}", stats.total_conns, stats.total_created);

    pool.close().await.unwrap();
    println!("Pool test completed");
}
