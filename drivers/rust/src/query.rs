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
        Ok(resp.data.unwrap_or(json!(null)))
    }

    /// Execute and decode the result into a typed value.
    pub async fn run_result<T: serde::de::DeserializeOwned>(&self, conn: &Connection) -> Result<T> {
        let data = self.run(conn).await?;
        serde_json::from_value(data).map_err(Error::Json)
    }

    // === Transformations ===

    /// Filter documents by predicate.
    pub fn filter(self, predicate: serde_json::Value) -> Query {
        Query::new(json!([term::FILTER, self.term, predicate]))
    }

    /// Order by field(s).
    pub fn order_by(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::ORDER_BY, self.term, fields]))
    }

    /// Limit results.
    pub fn limit(self, n: u64) -> Query {
        Query::new(json!([term::LIMIT, self.term, n]))
    }

    /// Skip results.
    pub fn skip(self, n: u64) -> Query {
        Query::new(json!([term::SKIP, self.term, n]))
    }

    /// Between range.
    pub fn between(self, lower: serde_json::Value, upper: serde_json::Value) -> Query {
        Query::new(json!([term::BETWEEN, self.term, lower, upper]))
    }

    /// Select specific fields.
    pub fn pluck(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.term, fields]))
    }

    /// Exclude specific fields.
    pub fn without(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::WITHOUT, self.term, fields]))
    }

    /// Merge additional fields.
    pub fn merge(self, other: serde_json::Value) -> Query {
        Query::new(json!([term::MERGE, self.term, other]))
    }

    /// Filter by field existence.
    pub fn has_fields(self, fields: Vec<String>) -> Query {
        Query::new(json!([term::HAS_FIELDS, self.term, fields]))
    }

    // === Mutations ===

    /// Update documents.
    pub fn update(self, changes: serde_json::Value) -> Query {
        Query::new(json!([term::UPDATE, self.term, changes]))
    }

    /// Delete documents.
    pub fn delete(self) -> Query {
        Query::new(json!([term::DELETE, self.term]))
    }

    /// Replace documents.
    pub fn replace(self, doc: serde_json::Value) -> Query {
        Query::new(json!([term::REPLACE, self.term, doc]))
    }

    // === Aggregations ===

    /// Count documents.
    pub fn count(self) -> Query {
        Query::new(json!([term::COUNT, self.term]))
    }

    /// Sum a field.
    pub fn sum(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::SUM, self.term, f])),
            None => Query::new(json!([term::SUM, self.term])),
        }
    }

    /// Average a field.
    pub fn avg(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::AVG, self.term, f])),
            None => Query::new(json!([term::AVG, self.term])),
        }
    }

    /// Min value.
    pub fn min(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::MIN, self.term, f])),
            None => Query::new(json!([term::MIN, self.term])),
        }
    }

    /// Max value.
    pub fn max(self, field: Option<String>) -> Query {
        match field {
            Some(f) => Query::new(json!([term::MAX, self.term, f])),
            None => Query::new(json!([term::MAX, self.term])),
        }
    }

    /// Group by field.
    pub fn group(self, field: String) -> Query {
        Query::new(json!([term::GROUP, self.term, field]))
    }

    /// Ungroup.
    pub fn ungroup(self) -> Query {
        Query::new(json!([term::UNGROUP, self.term]))
    }

    // === Changefeeds ===

    /// Subscribe to changes.
    pub fn changes(self) -> Query {
        Query::new(json!([term::CHANGES, self.term]))
    }
}

/// Table query with additional operations.
pub struct TableQuery {
    query: Query,
}

impl TableQuery {
    pub fn new(db_name: &str, table_name: &str) -> Self {
        Self {
            query: Query::new(json!([term::TABLE, [term::DB, db_name], table_name])),
        }
    }

    /// Get a document by primary key.
    pub fn get(self, id: &str) -> Query {
        Query::new(json!([term::GET, self.query.term, id]))
    }

    /// Get documents by secondary index keys.
    pub fn get_all(self, keys: Vec<serde_json::Value>) -> Query {
        let mut term = vec![json!(term::GET_ALL), self.query.term];
        term.extend(keys);
        Query::new(json!(term))
    }

    /// Insert document(s).
    pub fn insert(self, doc: serde_json::Value) -> Query {
        Query::new(json!([term::INSERT, self.query.term, doc]))
    }

    /// Create a secondary index.
    pub fn index_create(self, name: &str) -> Query {
        Query::new(json!([term::INDEX_CREATE, self.query.term, name]))
    }

    /// Drop a secondary index.
    pub fn index_drop(self, name: &str) -> Query {
        Query::new(json!([term::INDEX_DROP, self.query.term, name]))
    }

    /// List secondary indexes.
    pub fn index_list(self) -> Query {
        Query::new(json!([term::INDEX_LIST, self.query.term]))
    }

    /// Get the underlying query for chaining.
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

    /// Get a table reference.
    pub fn table(self, name: &str) -> TableQuery {
        TableQuery {
            query: Query::new(json!([term::TABLE, self.term, name])),
        }
    }

    /// Create a table.
    pub fn table_create(self, name: &str) -> Query {
        Query::new(json!([term::TABLE_CREATE, self.term, name]))
    }

    /// Drop a table.
    pub fn table_drop(self, name: &str) -> Query {
        Query::new(json!([term::TABLE_DROP, self.term, name]))
    }

    /// List tables.
    pub fn table_list(self) -> Query {
        Query::new(json!([term::TABLE_LIST, self.term]))
    }
}

/// Top-level R object (like rethinkdb's `r`).
pub struct R {
    db: String,
}

impl R {
    /// Create a new R with default database.
    pub fn new() -> Self {
        Self {
            db: "test".to_string(),
        }
    }

    /// Set the default database.
    pub fn use_db(mut self, db: &str) -> Self {
        self.db = db.to_string();
        self
    }

    /// Get a database reference.
    pub fn db(&self, name: &str) -> DbQuery {
        DbQuery::new(name)
    }

    /// Get a table reference using the default database.
    pub fn table(&self, name: &str) -> TableQuery {
        TableQuery::new(&self.db, name)
    }

    /// Create a database.
    pub fn db_create(&self, name: &str) -> Query {
        Query::new(json!([term::DB_CREATE, name]))
    }

    /// Drop a database.
    pub fn db_drop(&self, name: &str) -> Query {
        Query::new(json!([term::DB_DROP, name]))
    }

    /// List databases.
    pub fn db_list(&self) -> Query {
        Query::new(json!([term::DB_LIST]))
    }

    /// Wrap a value as a ReQL expression.
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
        let q = r.table("users")
            .into_query()
            .limit(10);
        let term = q.to_term();
        assert_eq!(term[0], json!(term::LIMIT));
        assert_eq!(term[2], json!(10));
    }

    #[test]
    fn test_skip_chain() {
        let r = R::new();
        let q = r.table("users")
            .into_query()
            .skip(5);
        let term = q.to_term();
        assert_eq!(term[0], json!(term::SKIP));
        assert_eq!(term[2], json!(5));
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
    fn test_update_query() {
        let r = R::new();
        let q = r.table("users")
            .get("abc")
            .update(json!({"name": "Jane"}));
        let term = q.to_term();
        assert_eq!(term[0], json!(term::UPDATE));
    }

    #[test]
    fn test_delete_query() {
        let r = R::new();
        let q = r.table("users").get("abc").delete();
        let term = q.to_term();
        assert_eq!(term[0], json!(term::DELETE));
    }

    #[test]
    fn test_count_query() {
        let r = R::new();
        let q = r.table("users").into_query().count();
        let term = q.to_term();
        assert_eq!(term[0], json!(term::COUNT));
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

    #[test]
    fn test_table_create() {
        let r = R::new();
        let q = r.db("test").table_create("users");
        let term = q.to_term();
        assert_eq!(term[0], json!(term::TABLE_CREATE));
    }

    #[test]
    fn test_changes() {
        let r = R::new();
        let q = r.table("users").into_query().changes();
        let term = q.to_term();
        assert_eq!(term[0], json!(term::CHANGES));
    }

    #[test]
    fn test_chained_operations() {
        let r = R::new();
        let q = r.table("users")
            .into_query()
            .filter(json!({"active": true}))
            .order_by(vec!["name".to_string()])
            .skip(10)
            .limit(5);
        let term = q.to_term();
        assert_eq!(term[0], json!(term::LIMIT));
    }

    #[test]
    fn test_use_db() {
        let r = R::new().use_db("production");
        let q = r.table("users");
        let query = q.into_query();
        let term = query.to_term();
        // term[1] should be [DB, "production"]
        let db_term = term[1].as_array().unwrap();
        assert_eq!(db_term[1], json!("production"));
    }

    #[test]
    fn test_index_operations() {
        let r = R::new();

        // IndexCreate
        let q = r.table("users").index_create("email");
        assert_eq!(q.to_term()[0], json!(term::INDEX_CREATE));

        // IndexList
        let q = r.table("users").index_list();
        assert_eq!(q.to_term()[0], json!(term::INDEX_LIST));

        // IndexDrop
        let q = r.table("users").index_drop("email");
        assert_eq!(q.to_term()[0], json!(term::INDEX_DROP));
    }
}
