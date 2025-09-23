package gaelogrus

import (
	"net/url"
	"time"

	"github.com/holyheld/gaelogrus/pool"
	"github.com/sirupsen/logrus"

	"encoding/json"
	"fmt"
)

var levelToSeverity = map[logrus.Level]severity{
	logrus.TraceLevel: sDEFAULT,
	logrus.DebugLevel: sDEBUG,
	logrus.InfoLevel:  sINFO,
	logrus.WarnLevel:  sWARNING,
	logrus.ErrorLevel: sERROR,
	logrus.FatalLevel: sCRITICAL,
	logrus.PanicLevel: sEMERGENCY,
}

type Formatter struct {
	projectID string
	logID     string
	pool      logrus.BufferPool
}

// GAEStandardFormatter returns a new Formatter.
func GAEStandardFormatter(options ...Option) *Formatter {
	f := Formatter{pool: pool.NewBufferPool(0)}
	for _, option := range options {
		option(&f)
	}

	return &f
}

// Option lets you configure the Formatter.
type Option func(*Formatter)

// WithProjectID lets you configure the GAE project for threaded messaging.
func WithProjectID(pid string) Option {
	return func(f *Formatter) {
		f.projectID = pid
	}
}

func WithLogID(lid string) Option {
	return func(f *Formatter) {
		f.logID = url.PathEscape(lid)
	}
}

func WithBufferPool(bp logrus.BufferPool) Option {
	return func(f *Formatter) {
		f.pool = bp
	}
}

// Format formats a logrus entry in Stackdriver format.
func (f *Formatter) Format(e *logrus.Entry) ([]byte, error) {
	cloned := e.Dup().Data

	// perform error type fix
	errV, exists := e.Data[logrus.ErrorKey]
	if exists {
		cloned[logrus.ErrorKey] = fmt.Sprint(errV)
	}

	cloned["message"] = e.Message
	reqV, _ := cloned["request"].(*httpRequestEntry)
	delete(cloned, "request")

	ee := logEntry{
		LogName:     fmt.Sprintf("projects/%s/logs/%s", f.projectID, f.logID),
		Severity:    string(levelToSeverity[e.Level]),
		JSONPayload: cloned,
		Timestamp:   e.Time.UTC().Format(time.RFC3339Nano),
		HTTPRequest: reqV,
	}

	if e.HasCaller() {
		ee.SourceLocation = &sourceLocationEntry{
			File:     e.Caller.File,
			Line:     fmt.Sprintf("%d", e.Caller.Line),
			Function: e.Caller.Function,
		}
	}

	xctc := TraceID(e.Context)
	if xctc != "" {
		traceID, spanID := ParseXCloudTraceContext(xctc)
		if traceID != "" && spanID != "" {
			ee.Trace = fmt.Sprintf("projects/%s/traces/%s", f.projectID, traceID)
			ee.SpanID = spanID
		}
	}

	buf := f.pool.Get()
	defer f.pool.Put(buf)

	err := json.NewEncoder(buf).Encode(ee)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	return buf.Bytes(), nil
}
