package telemetry

import (
	"context"
	"slices"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Context returns a zap field that attaches ctx to a log record, so the record carries the trace and span IDs of the span in ctx.
func Context(ctx context.Context) zap.Field {
	return zap.Field{Key: "context", Type: zapcore.SkipType, Interface: ctx}
}

// NewTraceIDCore wraps core to write the span context of a Context field as trace_id and span_id fields.
func NewTraceIDCore(core zapcore.Core) zapcore.Core {
	return &traceIDCore{Core: core}
}

type traceIDCore struct {
	zapcore.Core
	ctx context.Context //nolint:containedctx // carries the context field from With to Write
}

func (c *traceIDCore) With(fields []zapcore.Field) zapcore.Core {
	ctx, fields := splitContext(fields)
	if ctx == nil {
		ctx = c.ctx
	}
	return &traceIDCore{Core: c.Core.With(fields), ctx: ctx}
}

func (c *traceIDCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *traceIDCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	ctx, fields := splitContext(fields)
	if ctx == nil {
		ctx = c.ctx
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(slices.Clip(fields),
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	return c.Core.Write(ent, fields)
}

func splitContext(fields []zapcore.Field) (context.Context, []zapcore.Field) {
	i := slices.IndexFunc(fields, isContext)
	if i < 0 {
		return nil, fields
	}
	var ctx context.Context
	rest := fields[:i:i]
	for _, f := range fields[i:] {
		if c, ok := f.Interface.(context.Context); ok {
			ctx = c //nolint:fatcontext // the last context field wins, nothing is nested
			continue
		}
		rest = append(rest, f)
	}
	return ctx, rest
}

func isContext(f zapcore.Field) bool {
	_, ok := f.Interface.(context.Context)
	return ok
}
