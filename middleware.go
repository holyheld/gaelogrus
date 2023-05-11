package gaelogrus

import (
	"os"
	"runtime/debug"

	"github.com/sirupsen/logrus"

	"context"
	"fmt"
	"net/http"
	"time"
)

type LogContextKind string

const LogTraceContextKey LogContextKind = "logTraceID"
const LogEntryContextKey LogContextKind = "logEntry"

// XCloudTraceContext middleware extracts the X-Cloud-Trace-Context
// from the request header and injects it into the context. The value
// is used by package to group log entries by request.
func XCloudTraceContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), LogTraceContextKey, r.Header.Get("X-Cloud-Trace-Context"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AttachLogger bounds current global (std) logrus.Logger to the
// request context and injects contextualized logger into the context
func AttachLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		contextLogger := logrus.WithContext(ctx)
		ctx = context.WithValue(ctx, LogEntryContextKey, contextLogger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetLogger extracts contextualized logger from request context.
// If none found - it creates one on-place
func GetLogger(ctx context.Context) *logrus.Entry {
	entry, ok := ctx.Value(LogEntryContextKey).(*logrus.Entry)
	if !ok {
		return logrus.WithContext(ctx)
	}

	return entry
}

type responseData struct {
	status int
	size   int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.responseData.size += size
	return size, err
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.responseData.status = statusCode
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := GetLogger(ctx)

		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}

		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}

		defer func() {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}

			logger.WithFields(logrus.Fields{
				"ts":          start.UTC().Format(time.RFC1123),
				"http_scheme": scheme,
				"http_proto":  r.Proto,
				"http_method": r.Method,

				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),

				"uri":               fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI),
				"resp_status":       responseData.status,
				"resp_elapsed_ms":   float64(time.Since(start) / 1000000.0),
				"resp_bytes_length": responseData.size,
			}).Info("request completed")
		}()

		next.ServeHTTP(&lw, r)
	})
}

// Recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns a HTTP 500 (Internal Server Error) status if
// possible. Recoverer prints a request ID if one is provided.
func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {

				logEntity := GetLogger(r.Context())
				if logEntity != nil {
					debugStack := debug.Stack()
					s := prettyStack{}
					out, err := s.parse(debugStack, rvr)
					if err == nil {
						logEntity.Error(string(out))
					} else {
						// print stdlib output as a fallback
						os.Stderr.Write(debugStack)
					}
				}

				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
