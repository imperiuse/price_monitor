package helper

import (
	"context"

	"github.com/imperiuse/price_monitor/internal/uuid"
)

// The ctxKey type is unexported to prevent collisions with context keys defined in
// other packages.
type ctxKey int

// funcName is the context key for the handlerFuncName.  Its value of zero is
// arbitrary.  If this package defined other context keys, they would have
// different integer values.
const (
	funcName ctxKey = iota
	uuidValue
	ctxTTL
)

// NewContextWithFuncName return ctx which kept func name in values.
func NewContextWithFuncName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, funcName, name)
}

// NewContextWithUUID return ctx which kept uuid msg in values.
func NewContextWithUUID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, uuidValue, uid)
}

// NewContextWithCtxTTL return ctx which kept ctxTTL msg in values.
func NewContextWithCtxTTL(ctx context.Context, ttl uint64) context.Context {
	return context.WithValue(ctx, ctxTTL, ttl)
}

// FromContextGetFuncName return value of func name from context.
func FromContextGetFuncName(ctx context.Context) string {
	// ctx.Value returns nil if ctx has no value for the key;
	// the net.IP type assertion returns ok=false for nil.
	v, found := ctx.Value(funcName).(string)
	if !found {
		return ""
	}

	return v
}

// FromContextGetUUID return value of uuid from context if key does not exist return new UUID4.
func FromContextGetUUID(ctx context.Context) string {
	// ctx.Value returns nil if ctx has no value for the key;
	// the net.IP type assertion returns ok=false for nil.
	v, found := ctx.Value(uuidValue).(string)
	if !found {
		return uuid.UUID4()
	}

	return v
}

// FromContextGetCtxTTL return value of ctx TTL from context.
func FromContextGetCtxTTL(ctx context.Context) uint64 {
	// ctx.Value returns nil if ctx has no value for the key;
	// the net.IP type assertion returns ok=false for nil.
	v, found := ctx.Value(ctxTTL).(uint64)
	if !found {
		return 0
	}

	return v
}
