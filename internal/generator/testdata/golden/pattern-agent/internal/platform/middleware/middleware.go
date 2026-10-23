package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"example.com/sample-app/internal/platform/logging"
	"example.com/sample-app/internal/platform/respond"
)

type requestKey struct{}

func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestKey{}).(string); return id }
func Logger(ctx context.Context, base logging.Logger) logging.Logger {
	return base.With("request_id", RequestID(ctx))
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func Wrap(next http.Handler, log logging.Logger, maxBody int64, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			respond.Problem(w, http.StatusInternalServerError)
			return
		}
		id := hex.EncodeToString(bytes[:])
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), requestKey{}, id), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		scoped := Logger(ctx, log)
		w.Header().Set("X-Request-ID", id)
		rw := &responseWriter{ResponseWriter: w}
		start := time.Now()
		defer func() {
			abort := false
			if failure := recover(); failure != nil {
				scoped.Error("request panic")
				if rw.status == 0 {
					respond.Problem(rw, http.StatusInternalServerError)
				} else {
					abort = true
				}
			}
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			scoped.Info("request", "method", r.Method, "status", status, "duration", time.Since(start))
			if abort {
				panic(http.ErrAbortHandler)
			}
		}()
		if r.ContentLength > maxBody {
			respond.Problem(rw, http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(rw, r.Body, maxBody)
		next.ServeHTTP(rw, r)
	})
}
