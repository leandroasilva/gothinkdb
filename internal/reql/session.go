package reql

import "context"

// Authorizer decides whether the current connection may perform an operation on
// a database. It returns nil when allowed, or an error describing the denial.
// The driver (ReQL) path injects an authorizer built from the authenticated
// user's per-database permissions; the admin/HTTP path may omit it (admin has
// full access).
type Authorizer func(db string, read, write, create, drop bool) error

type sessionKey int

const (
	defaultDBKey sessionKey = iota
	authorizerKey
)

// WithSession returns a ctx carrying the connection's default database and the
// authorizer used to enforce per-database access during evaluation. This
// replaces the previous global Evaluator.currentDB so concurrent connections
// are isolated from each other.
func WithSession(ctx context.Context, defaultDB string, authz Authorizer) context.Context {
	if defaultDB != "" {
		ctx = context.WithValue(ctx, defaultDBKey, defaultDB)
	}
	if authz != nil {
		ctx = context.WithValue(ctx, authorizerKey, authz)
	}
	return ctx
}

// DefaultDBFromContext returns the connection's default database, if set.
func DefaultDBFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(defaultDBKey).(string)
	return v, ok
}

// AuthorizerFromContext returns the authorizer bound to the ctx, if any.
func AuthorizerFromContext(ctx context.Context) (Authorizer, bool) {
	v, ok := ctx.Value(authorizerKey).(Authorizer)
	return v, ok
}

// resolveDB picks the effective database: an explicit name wins, otherwise the
// connection's default DB from ctx, otherwise the evaluator's current DB.
func (e *Evaluator) resolveDB(ctx context.Context, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if db, ok := DefaultDBFromContext(ctx); ok && db != "" {
		return db
	}
	return e.currentDB
}

// authorize enforces per-database access when an authorizer is present in ctx.
func (e *Evaluator) authorize(ctx context.Context, db string, read, write, create, drop bool) error {
	authz, ok := AuthorizerFromContext(ctx)
	if !ok || authz == nil {
		return nil
	}
	return authz(db, read, write, create, drop)
}
