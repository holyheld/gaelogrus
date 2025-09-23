package gaelogrus

import (
	"runtime/debug"

	"github.com/sirupsen/logrus"

	"context"
	"fmt"
	"net/http"
	"time"
)

type LogContextKind string

const (
	LogTraceContextKey LogContextKind = "logTraceID"
	LogEntryContextKey LogContextKind = "logEntry"
)

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

			logger.WithField("request", &httpRequestEntry{
				RequestMethod: r.Method,
				RequestURL:    fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI),
				RequestSize:   fmt.Sprintf("%d", r.ContentLength),
				Status:        responseData.status,
				ResponseSize:  fmt.Sprintf("%d", responseData.size),
				UserAgent:     r.UserAgent(),
				RemoteIP:      r.RemoteAddr,
				Referer:       r.Referer(),
				Latency:       time.Since(start).String(),
				Protocol:      r.Proto,
			}).Log(getLogLevel(r, responseData.status), "request completed")
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
			rvr := recover()
			if rvr == nil {
				return
			}

			if rvr == http.ErrAbortHandler {
				// we don't recover http.ErrAbortHandler so the response
				// to the client is aborted, this should not be logged
				panic(rvr)
			}

			debugStack := debug.Stack()
			s := prettyStack{}
			out, err := s.parse(debugStack, rvr)
			if err != nil {
				out = debugStack
			}

			logEntity := GetLogger(r.Context())
			if logEntity != nil {
				logEntity.Error(string(out))
			} else {
				recovererErrorWriter.Write(out)
			}

			if r.Header.Get("Connection") != "Upgrade" {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func getLogLevel(r *http.Request, status int) logrus.Level {
	if status == 0 {
		return logrus.WarnLevel
	}

	switch r.Method {
	case http.MethodHead:
		return logrus.TraceLevel
	case http.MethodConnect:
		return logrus.TraceLevel
	case http.MethodOptions:
		return logrus.TraceLevel
	case http.MethodTrace:
		return logrus.TraceLevel
	}

	if status < 200 {
		return logrus.TraceLevel
	}

	if status < 400 {
		return logrus.InfoLevel
	}

	if status < 500 {
		return logrus.WarnLevel
	}

	return logrus.ErrorLevel
}
