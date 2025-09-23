package gaelogrus

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

type severity string

// The severity of the event described in a log entry.
// See https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
const (
	sDEFAULT   severity = "DEFAULT"
	sDEBUG     severity = "DEBUG"
	sINFO      severity = "INFO"
	sWARNING   severity = "WARNING"
	sERROR     severity = "ERROR"
	sCRITICAL  severity = "CRITICAL"
	sEMERGENCY severity = "EMERGENCY"
)

// TraceID returns the trace ID value from the context.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	trace, ok := ctx.Value(LogTraceContextKey).(string)
	if !ok {
		return ""
	}
	return trace
}

type httpRequestEntry struct {
	RequestMethod string `json:"requestMethod,omitempty"`
	RequestURL    string `json:"requestUrl,omitempty"`
	RequestSize   string `json:"requestSize,omitempty"`
	Status        int    `json:"status,omitempty"`
	ResponseSize  string `json:"responseSize,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
	RemoteIP      string `json:"remoteIp,omitempty"`
	ServerIP      string `json:"serverIp,omitempty"`
	Referer       string `json:"referer,omitempty"`
	Latency       string `json:"latency,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type sourceLocationEntry struct {
	File     string `json:"file,omitempty"`
	Line     string `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// this is JSON format for producing logs via stdout, but keeping them grouped by request
type logEntry struct {
	LogName        string               `json:"logName"`
	Trace          string               `json:"logging.googleapis.com/trace,omitempty"`
	SpanID         string               `json:"logging.googleapis.com/spanId,omitempty"`
	Severity       string               `json:"severity,omitempty"`
	JSONPayload    interface{}          `json:"jsonPayload,omitempty"`
	HTTPRequest    *httpRequestEntry    `json:"httpRequest,omitempty"`
	Timestamp      string               `json:"timestamp"`
	SourceLocation *sourceLocationEntry `json:"logging.googleapis.com/sourceLocation,omitempty"`
}

// ParseXCloudTraceContext parses provided header value in format of "X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=TRACE_TRUE"
// and returns traceID and spanID if present
//
// `TRACE_ID` is a 32-character hexadecimal value representing a 128-bit
// number. It should be unique between your requests, unless you
// intentionally want to bundle the requests together. You can use UUIDs.
//
// `SPAN_ID` is the decimal representation of the (unsigned) span ID. It
// should be 0 for the first span in your trace. For subsequent requests,
// set SPAN_ID to the span ID of the parent request. See the description
// of TraceSpan (REST, RPC) for more information about nested traces.
//
// `TRACE_TRUE` must be 1 to trace this request. Specify 0 to not trace the
// request.
func ParseXCloudTraceContext(t string) (traceID, spanID string) {
	if t == "" {
		return "", ""
	}

	// 32 characters plus 1 (forward slash) plus 1 (at least one decimal
	// representing the span).
	if len(t) < 34 {
		return "", ""
	}

	// The first character after the TRACE_ID should be a forward slash.
	if t[32] != '/' {
		return "", ""
	}

	// handle "TRACE_ID/SPAN_ID" missing the ";o=1" part.
	last := strings.LastIndex(t, ";")
	if last == -1 {
		return t[0:32], t[33:]
	}
	return t[0:32], t[33:last]
}

func GenerateSubTrace(ctx context.Context) string {
	xctc := TraceID(ctx)
	if xctc != "" {
		traceID, spanID := ParseXCloudTraceContext(xctc)
		newSpanID := generateDifferentSpanID(spanID, 0)
		return fmt.Sprintf("%s/%s", traceID, newSpanID)
	}

	return ""
}

func generateDifferentSpanID(previous string, attempts int) string {
	if attempts > 30 {
		return "1"
	}

	i := rand.Int63()
	s := strconv.FormatInt(i, 10)
	if s == previous {
		return generateDifferentSpanID(previous, attempts+1)
	}

	return s
}
