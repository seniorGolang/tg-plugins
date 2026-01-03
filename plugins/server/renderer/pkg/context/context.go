package context

import (
	"context"
	"reflect"
	"time"
)

type contextKey string
type Context = context.Context
type CancelFunc = context.CancelFunc

var TODO = context.TODO
var Canceled = context.Canceled
var Background = context.Background

func WithCtx[T any](ctx context.Context, value T) context.Context {
	return context.WithValue(ctx, contextKey(reflect.TypeOf(value).String()), value)
}

func FromCtx[T any](ctx context.Context, defaults ...T) (value T) {

	var ok bool
	if value, ok = ctx.Value(contextKey(reflect.TypeOf(value).String())).(T); !ok {
		if len(defaults) != 0 {
			value = defaults[0]
		}
	}
	return
}

func WithTimeout(parent context.Context, timeout time.Duration) (Context, CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

func WithCancel(parent context.Context) (Context, CancelFunc) {
	return context.WithCancel(parent)
}
