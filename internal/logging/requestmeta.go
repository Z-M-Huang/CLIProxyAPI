package logging

import (
	"context"
	"sync/atomic"
)

type endpointKey struct{}
type responseStatusKey struct{}

type responseStatusHolder struct {
	status atomic.Int32
}

func WithEndpoint(ctx context.Context, endpoint string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, endpointKey{}, endpoint)
}

func GetEndpoint(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if endpoint, ok := ctx.Value(endpointKey{}).(string); ok {
		return endpoint
	}
	return ""
}

func WithResponseStatusHolder(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if holder, ok := ctx.Value(responseStatusKey{}).(*responseStatusHolder); ok && holder != nil {
		return ctx
	}
	return context.WithValue(ctx, responseStatusKey{}, &responseStatusHolder{})
}

func SetResponseStatus(ctx context.Context, status int) {
	if ctx == nil || status <= 0 {
		return
	}
	holder, ok := ctx.Value(responseStatusKey{}).(*responseStatusHolder)
	if !ok || holder == nil {
		return
	}
	holder.status.Store(int32(status))
}

func GetResponseStatus(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	holder, ok := ctx.Value(responseStatusKey{}).(*responseStatusHolder)
	if !ok || holder == nil {
		return 0
	}
	return int(holder.status.Load())
}

// CopyMetadata returns dst extended with the request-scoped observability
// metadata carried by src (endpoint, response-status holder). Callers use
// this to hand off metadata to a long-lived async context without retaining
// short-lived parent values such as a Gin handler's *gin.Context, which can
// be recycled by Gin's pool after the request returns.
//
// The response-status holder is shared by pointer, so writes from the
// request goroutine remain visible to async readers via the returned ctx.
//
// request_id is intentionally NOT copied; callers commonly need to fall
// back to a Gin request ID when the standard context lacks one and should
// call WithRequestID separately after composing CopyMetadata.
func CopyMetadata(src, dst context.Context) context.Context {
	if dst == nil {
		dst = context.Background()
	}
	if src == nil {
		return dst
	}
	if endpoint, ok := src.Value(endpointKey{}).(string); ok && endpoint != "" {
		dst = context.WithValue(dst, endpointKey{}, endpoint)
	}
	if holder, ok := src.Value(responseStatusKey{}).(*responseStatusHolder); ok && holder != nil {
		dst = context.WithValue(dst, responseStatusKey{}, holder)
	}
	return dst
}
