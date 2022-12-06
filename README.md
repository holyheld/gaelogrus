# gaelogrus

`gaelogrus` is the wrapper around [standard logrus](https://github.com/sirupsen/logrus) package that includes utilities
to support Google Cloud Stackdriver (logging tool) when application runs in Google App Engine

## Installation
* `go get -u github.com/viaMover/gaelogrus`

## Usage notes
```go
package main

import (
    "github.com/sirupsen/logrus"
    "github.com/viaMover/gaelogrus"
    
    "net/http"
)

func main() {
    // set logrus formatter 
    logrus.SetFormatter(gaelogrus.GAEStandardFormatter(gaelogrus.WithProjectID("project_id")))
    
    r := chi.NewRouter() 
    // attach middlewares to the router
    r.Use(gaelogrus.XCloudTraceContext, gaelogrus.AttachLogger)
    // add request logger if needed 
    r.Use(gaelogrus.RequestLogger)
	r.Get("/", HelloWorld)
}

func HelloWorld(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    // retrieve the logger from context
    logger := gaelorgus.GetLogger(ctx)

    // use it in the handlers as needed
    logger.Infof("Hello from logger!")

    _, _ = w.Write([]byte("hello world"))
    w.WriteHeader(http.StatusOK)
}
```

