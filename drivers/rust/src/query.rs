use serde_json::json;
use crate::connection::Connection;
use crate::types::*;

/// A ReQL query builder.
#[derive(Debug, Clone)]
pub struct Query {
    term: serde_json::Value,
}

impl Query {
    /// Create a new query from a term.
    pub fn new(term: serde_json::Value) -> Self {
        Self { term }
    }

    /// Get the raw term.
    pub fn to_term(&self) -> &serde_json::Value {
        &self.term
    }

    /// Execute the query on a connection.
    pub async fn run(&self, conn: &Connection) -> Result<serde_json::Value> {
        let resp = conn.query(self.term.clone()).await?;
        let data = resp.data.unwrap_or(json!(null));
        // The server wraps all responses in an array (RethinkDB convention).
        // Unwrap single-element arrays.
        if let Some(arr) = data.as_array() {
            if arr.len() == 1 {
                return Ok(arr[0].clone());
            }
        }
        Ok(data)
    }

    /// Execute and decode the result into a typed value.
    pub async fn run_result<T: serde::de::DeserializeOwned>(&self, conn: &Connection) -> Result<T> {
        let data = self.run(conn).await?;
        serde_json::from_value(data).map_err(Error::Json)
    }

    // === Transformations ===

    pub fn filter(self, predicate: serde_json::Value) -> Query {
        Query::new(json!([term::FILTER, self.term, predicate]))
    }

    pub fn order_by(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::ORDER_BY, self.term, fields]))
    }

    pub fn limit(self, n: u64) -> Query {
        Query::new(json!([term::LIMIT, self.term, n]))
    }

    pub fn skip(self, n: u64) -> Query {
        Query::new(json!([term::SKIP, self.term, n]))
    }

    pub fn between(self, index: &str, lower: serde_json::Value, upper: serde_json::Value) -> Query {
        Query::new(json!([term::BETWEEN, self.term, index, lower, upper]))
    }

    pub fn pluck(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.term, fields]))
    }

    pub fn without(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::WITHOUT, self.term, fields]))
    }

    pub fn merge(self, other: serde_json::Value) -> Query {
        Query::new(json!([term::MERGE, self.term, other]))
    }

    pub fn has_fields(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.term, fields]))
    }

    pub fn has_fields(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.term, fields]))
    }

    pub fn without(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::WITHOUT, self.term, fields]))
    }

    pub fn merge(self, other: serde_json::Value) -> Query {
        Query::new(json!([term::MERGE, self.term, other]))
    }

    pub fn inner_join(self, other: serde_json::Value, predicate: serde_json::Value) -> Query {
        Query::new(json!([term::INNER_JOIN, self.term, other, predicate]))
    }

    pub fn outer_join(self, other: serde_json::Value, predicate: serde_json::Value) -> Query {
        Query::new(json!([term::OUTER_JOIN, self.term, other, predicate]))
    }

    // === Mutations ===

    pub fn update(self, changes: serde_json::Value) -> Query {
        Query::new(json!([term::UPDATE, self.term, changes]))
    }

    pub fn delete(self) -> Query {
        Query::new(json!([term::DELETE, self.term]))
    }

    pub fn replace(self, doc: serde_json::Value) -> Query {
        Query::new(json!([term::REPLACE, self.term, doc]))
    }

    // === Aggregations ===

    pub fn count(self) -> Query {
        Query::new(json!([term::COUNT, self.term]))
    }

    pub fn sum(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::SUM, self.term, f])),
            None => Query::new(json!([term::SUM, self.term])),
        }
    }

    pub fn avg(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::AVG, self.term, f])),
            None => Query::new(json!([term::AVG, self.term])),
        }
    }

    pub fn min(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::MIN, self.term, f])),
            None => Query::new(json!([term::MIN, self.term])),
        }
    }

    pub fn max(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::MAX, self.term, f])),
            None => Query::new(json!([term::MAX, self.term])),
        }
    }

    pub fn group(self, field: String) -> Query {
        Query::new(json!([term::GROUP, self.term, field]))
    }

    pub fn ungroup(self) -> Query {
        Query::new(json!([term::UNGROUP, self.term]))
    }

    // === Changefeeds ===

    pub fn changes(self) -> Query {
        Query::new(json!([term::CHANGES, self.term]))
    }
}

/// Table query with additional operations.
pub struct TableQuery {
    query: Query,
}

impl TableQuery {
    pub fn new(table_name: &str) -> Self {
        Self {
            query: Query::new(json!([term::TABLE, table_name])),
        }
    }

    pub fn get(self, id: &str) -> Query {
        Query::new(json!([term::GET, self.query.term, id]))
    }

    pub fn get_all(self, keys: Vec<serde_json::Value>) -> Query {
        let mut term = vec![json!(term::GET_ALL), self.query.term];
        term.extend(keys);
        Query::new(json!(term))
    }

    pub fn insert(self, doc: serde_json::Value) -> Query {
        Query::new(json!([term::INSERT, self.query.term, doc]))
    }

    pub fn update(self, changes: serde_json::Value) -> Query {
        Query::new(json!([term::UPDATE, self.query.term, changes]))
    }

    pub fn delete(self) -> Query {
        Query::new(json!([term::DELETE, self.query.term]))
    }

    pub fn filter(self, predicate: serde_json::Value) -> Query {
        Query::new(json!([term::FILTER, self.query.term, predicate]))
    }

    pub fn order_by(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::ORDER_BY, self.query.term, fields]))
    }

    pub fn limit(self, n: u64) -> Query {
        Query::new(json!([term::LIMIT, self.query.term, n]))
    }

    pub fn skip(self, n: u64) -> Query {
        Query::new(json!([term::SKIP, self.query.term, n]))
    }

    pub fn count(self) -> Query {
        Query::new(json!([term::COUNT, self.query.term]))
    }

    pub fn sum(self, field: &str) -> Query {
        Query::new(json!([term::SUM, self.query.term, field]))
    }

    pub fn avg(self, field: &str) -> Query {
        Query::new(json!([term::AVG, self.query.term, field]))
    }

    pub fn min(self, field: &str) -> Query {
        Query::new(json!([term::MIN, self.query.term, field]))
    }

    pub fn max(self, field: &str) -> Query {
        Query::new(json!([term::MAX, self.query.term, field]))
    }

    pub fn group(self, field: &str) -> Query {
        Query::new(json!([term::GROUP, self.query.term, field]))
    }

    pub fn has_fields(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.query.term, fields]))
    }

    pub fn without(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::WITHOUT, self.query.term, fields]))
    }

    pub fn index_create(self, name: &str) -> Query {
        Query::new(json!([term::INDEX_CREATE, self.query.term, name]))
    }

    pub fn index_drop(self, name: &str) -> Query {
        Query::new(json!([term::INDEX_DROP, self.query.term, name]))
    }

    pub fn index_list(self) -> Query {
        Query::new(json!([term::INDEX_LIST, self.query.term]))
    }

    pub fn into_query(self) -> Query {
        self.query
    }
}

/// Database query.
pub struct DbQuery {
    term: serde_json::Value,
}

impl DbQuery {
    pub fn new(name: &str) -> Self {
        Self {
            term: json!([term::DB, name]),
        }
    }

    pub fn table(self, name: &str) -> TableQuery {
        TableQuery::new(name)
    }

    pub fn table_create(self, name: &str) -> Query {
        Query::new(json!([term::TABLE_CREATE, name, self.term]))
    }

    pub fn table_drop(self, name: &str) -> Query {
        Query::new(json!([term::TABLE_DROP, name, self.term]))
    }

    pub fn table_list(self) -> Query {
        Query::new(json!([term::TABLE_LIST, self.term]))
    }
}

/// Top-level R object.
pub struct R {
    db: String,
}

impl R {
    pub fn new() -> Self {
        Self { db: "test".to_string() }
    }

    pub fn use_db(mut self, db: &str) -> Self {
        self.db = db.to_string();
        self
    }

    pub fn db(&self, name: &str) -> DbQuery {
        DbQuery::new(name)
    }

    pub fn table(&self, name: &str) -> TableQuery {
        TableQuery::new(name)
    }

    pub fn db_create(&self, name: &str) -> Query {
        Query::new(json!([term::DB_CREATE, name]))
    }

    pub fn db_drop(&self, name: &str) -> Query {
        Query::new(json!([term::DB_DROP, name]))
    }

    pub fn db_list(&self) -> Query {
        Query::new(json!([term::DB_LIST]))
    }

    pub fn expr(&self, value: serde_json::Value) -> Query {
        Query::new(json!([term::DATUM, value]))
    }
}

impl Default for R {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_table_query() {
        let r = R::new();
        let q = r.table("users");
        let query = q.into_query();
        let term = query.to_term();
        assert_eq!(term[0], json!(term::TABLE));
    }

    #[test]
    fn test_db_query() {
        let r = R::new();
        let q = r.db("mydb");
        let term = q.term;
        assert_eq!(term[0], json!(term::DB));
        assert_eq!(term[1], json!("mydb"));
    }

    #[test]
    fn test_filter_chain() {
        let r = R::new();
        let q = r.table("users")
            .into_query()
            .filter(json!({"active": true}));
        let term = q.to_term();
        assert_eq!(term[0], json!(term::FILTER));
    }

    #[test]
    fn test_limit_chain() {
        let r = R::new();
        let q = r.table("users").into_query().limit(10);
        let term = q.to_term();
        assert_eq!(term[0], json!(term::LIMIT));
        assert_eq!(term[2], json!(10));
    }

    #[test]
    fn test_get_query() {
        let r = R::new();
        let q = r.table("users").get("abc123");
        let term = q.to_term();
        assert_eq!(term[0], json!(term::GET));
        assert_eq!(term[2], json!("abc123"));
    }

    #[test]
    fn test_insert_query() {
        let r = R::new();
        let q = r.table("users").insert(json!({"name": "John"}));
        let term = q.to_term();
        assert_eq!(term[0], json!(term::INSERT));
    }

    #[test]
    fn test_db_list() {
        let r = R::new();
        let q = r.db_list();
        let term = q.to_term();
        assert_eq!(*term, json!([term::DB_LIST]));
    }

    #[test]
    fn test_db_create() {
        let r = R::new();
        let q = r.db_create("mydb");
        let term = q.to_term();
        assert_eq!(*term, json!([term::DB_CREATE, "mydb"]));
    }
}
