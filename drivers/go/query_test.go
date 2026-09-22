package gothinkdb

import (
	"testing"
)

func TestNewR(t *testing.T) {
	r := NewR()
	if r == nil {
		t.Fatal("NewR returned nil")
	}
}

func TestTableQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users")
	if q == nil {
		t.Fatal("Table returned nil")
	}
	term := q.ToTerm()
	if len(term) != 2 {
		t.Fatalf("expected 2 elements in term, got %d", len(term))
	}
	if term[0] != TermTable {
		t.Errorf("expected term[0] = %d (TABLE), got %v", TermTable, term[0])
	}
}

func TestDbQuery(t *testing.T) {
	r := NewR()
	q := r.DB("mydb")
	if q == nil {
		t.Fatal("DB returned nil")
	}
	term := q.term
	if len(term) != 2 {
		t.Fatalf("expected 2 elements in term, got %d", len(term))
	}
	if term[0] != TermDB {
		t.Errorf("expected term[0] = %d (DB), got %v", TermDB, term[0])
	}
	if term[1] != "mydb" {
		t.Errorf("expected term[1] = 'mydb', got %v", term[1])
	}
}

func TestFilterChain(t *testing.T) {
	r := NewR()
	q := r.Table("users").Filter(map[string]interface{}{"active": true})
	term := q.ToTerm()
	if term[0] != TermFilter {
		t.Errorf("expected FILTER term, got %v", term[0])
	}
}

func TestLimitChain(t *testing.T) {
	r := NewR()
	q := r.Table("users").Limit(10)
	term := q.ToTerm()
	if term[0] != TermLimit {
		t.Errorf("expected LIMIT term, got %v", term[0])
	}
	if term[2] != 10 {
		t.Errorf("expected limit 10, got %v", term[2])
	}
}

func TestSkipChain(t *testing.T) {
	r := NewR()
	q := r.Table("users").Skip(5)
	term := q.ToTerm()
	if term[0] != TermSkip {
		t.Errorf("expected SKIP term, got %v", term[0])
	}
	if term[2] != 5 {
		t.Errorf("expected skip 5, got %v", term[2])
	}
}

func TestGetQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Get("abc123")
	term := q.ToTerm()
	if term[0] != TermGet {
		t.Errorf("expected GET term, got %v", term[0])
	}
	if term[2] != "abc123" {
		t.Errorf("expected id 'abc123', got %v", term[2])
	}
}

func TestInsertQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Insert(map[string]interface{}{"name": "John"})
	term := q.ToTerm()
	if term[0] != TermInsert {
		t.Errorf("expected INSERT term, got %v", term[0])
	}
}

func TestUpdateQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Get("abc").Update(map[string]interface{}{"name": "Jane"})
	term := q.ToTerm()
	if term[0] != TermUpdate {
		t.Errorf("expected UPDATE term, got %v", term[0])
	}
}

func TestDeleteQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Get("abc").Delete()
	term := q.ToTerm()
	if term[0] != TermDelete {
		t.Errorf("expected DELETE term, got %v", term[0])
	}
}

func TestCountQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Count()
	term := q.ToTerm()
	if term[0] != TermCount {
		t.Errorf("expected COUNT term, got %v", term[0])
	}
}

func TestDBListQuery(t *testing.T) {
	r := NewR()
	q := r.DBList()
	term := q.ToTerm()
	if len(term) != 1 || term[0] != TermDBList {
		t.Errorf("expected [59], got %v", term)
	}
}

func TestDBCreateQuery(t *testing.T) {
	r := NewR()
	q := r.DBCreate("mydb")
	term := q.ToTerm()
	if len(term) != 2 || term[0] != TermDBCreate || term[1] != "mydb" {
		t.Errorf("expected [57, 'mydb'], got %v", term)
	}
}

func TestTableCreateQuery(t *testing.T) {
	r := NewR()
	q := r.DB("test").TableCreate("users")
	term := q.ToTerm()
	if term[0] != TermTableCreate {
		t.Errorf("expected TABLE_CREATE term, got %v", term[0])
	}
}

func TestChangesQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").Changes()
	term := q.ToTerm()
	if term[0] != TermChanges {
		t.Errorf("expected CHANGES term, got %v", term[0])
	}
}

func TestChainedQuery(t *testing.T) {
	r := NewR()
	q := r.Table("users").
		Filter(map[string]interface{}{"active": true}).
		OrderBy("name").
		Skip(10).
		Limit(5)
	term := q.ToTerm()
	// Should be nested: LIMIT(SKIP(ORDER_BY(FILTER(TABLE, ...), ...), ...), ...)
	if term[0] != TermLimit {
		t.Errorf("expected outermost LIMIT, got %v", term[0])
	}
}

func TestUseDB(t *testing.T) {
	r := NewR()
	r.Use("production")
	q := r.Table("users")
	term := q.ToTerm()
	// Table now just uses [TABLE, "users"] without DB reference
	if term[0] != TermTable {
		t.Errorf("expected TABLE term, got %v", term[0])
	}
	if term[1] != "users" {
		t.Errorf("expected table 'users', got %v", term[1])
	}
}

func TestIndexOperations(t *testing.T) {
	r := NewR()

	// IndexCreate
	q := r.Table("users").IndexCreate("email")
	term := q.ToTerm()
	if term[0] != TermIndexCreate {
		t.Errorf("expected INDEX_CREATE, got %v", term[0])
	}

	// IndexList
	q = r.Table("users").IndexList()
	term = q.ToTerm()
	if term[0] != TermIndexList {
		t.Errorf("expected INDEX_LIST, got %v", term[0])
	}

	// IndexDrop
	q = r.Table("users").IndexDrop("email")
	term = q.ToTerm()
	if term[0] != TermIndexDrop {
		t.Errorf("expected INDEX_DROP, got %v", term[0])
	}
}
