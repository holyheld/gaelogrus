package gaelogrus

import (
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
}

// GAEStandardFormatter returns a new Formatter.
func GAEStandardFormatter(options ...Option) *Formatter {
	f := Formatter{}
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

// Format formats a logrus entry in Stackdriver format.
func (f *Formatter) Format(e *logrus.Entry) ([]byte, error) {
	// perform error type fix
	errV, exists := e.Data[logrus.ErrorKey]
	if exists {
		e.Data[logrus.ErrorKey] = fmt.Sprint(errV)
	}
	ee := logEntry{
		Severity: string(levelToSeverity[e.Level]),
		Message:  e.Message,
		Data:     e.Data,
	}

	xctc := TraceID(e.Context)
	if xctc != "" {
		traceID, spanID := ParseXCloudTraceContext(xctc)
		if traceID != "" && spanID != "" {
			ee.Trace = fmt.Sprintf("projects/%s/traces/%s", f.projectID, traceID)
			ee.SpanID = spanID
		}
	}

	userID, ok := e.Context.Value(LogUserIDContextKey).(string)
	if ok {
		ee.UserID = userID
	}

	ee.UserMeta = e.Context.Value(LogUserMetaContextKey)

	b, err := json.Marshal(ee)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
