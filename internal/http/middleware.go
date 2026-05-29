package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxLogger
	ctxUserID
	ctxJFUserID
)

func RequestID(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logging(base *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			start := time.Now()
			rid, _ := r.Context().Value(ctxRequestID).(string)
			logger := base.With("request_id", rid)
			ctx := context.WithValue(r.Context(), ctxLogger, logger)

			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r.WithContext(ctx))

			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"latency_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type statusRecorder struct {
	stdhttp.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func requestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}
