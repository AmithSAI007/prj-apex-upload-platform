// Package constants defines typed context keys used across the application
// to store and retrieve request-scoped values such as user identity and
// trace correlation IDs. Using a dedicated type (ctxKey) prevents collisions
// with context keys from other packages.
package constants

// ctxKey is a private string type that prevents context-key collisions with
// other packages that might use plain string keys.
type ctxKey string

const (
	// CtxUserIDKey stores the authenticated user's ID in the request context.
	// Set by the auth middleware after successful JWT validation.
	CtxUserIDKey ctxKey = "user_id"

	// CtxTenantIDKey stores the tenant ID extracted from the request context.
	// Used for multi-tenant data isolation in Firestore queries.
	CtxTenantIDKey ctxKey = "tenant_id"

	// CtxTraceIDKey stores the application-level correlation ID (distinct from
	// OpenTelemetry trace IDs) for request logging and error responses.
	CtxTraceIDKey ctxKey = "trace_id"
)
