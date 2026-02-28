package constants

type ctxKey string

const (
	CtxUserIDKey   ctxKey = "user_id"
	CtxTenantIDKey ctxKey = "tenant_id"
	CtxTraceIDKey  ctxKey = "trace_id"
)
