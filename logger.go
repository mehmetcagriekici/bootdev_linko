package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"boot.dev/linko/internal/linkoerr"
	pkgerr "github.com/pkg/errors"
)

type closeFunc func() error

type stackTracer interface {
	error
	StackTrace() pkgerr.StackTrace
}

type multiError interface {
	error
	Unwrap() []error
}

func errorAttrs(err error) []slog.Attr {
	attrs := []slog.Attr{
		{Key: "message", Value: slog.StringValue(err.Error())},
	}
	attrs = append(attrs, linkoerr.Attrs(err)...)
	if stackErr, ok := errors.AsType[stackTracer](err); ok {
		attrs = append(attrs, slog.Attr{
			Key:   "stack_trace",
			Value: slog.StringValue(fmt.Sprintf("%+v", stackErr.StackTrace())),
		})
	}
	return attrs
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		if multiErr, ok := errors.AsType[multiError](err); ok {
			var errAttrs []slog.Attr
			for i, e := range multiErr.Unwrap() {
				errAttrs = append(errAttrs, slog.GroupAttrs(fmt.Sprintf("error_%d", i+1), errorAttrs(e)...))
			}
			return slog.GroupAttrs("errors", errAttrs...)
		}

		return slog.GroupAttrs("error", errorAttrs(err)...)
	}
	return a
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			logger.Info("Served request", 
				"method", r.Method, 
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr,
				)
		})
	}
}


func initializeLogger() (*slog.Logger, closeFunc, error) {
  logFile, err := os.OpenFile(os.Getenv("LINKO_LOG_FILE"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
  if err != nil {
	  return nil, nil, err
  }

	bufferedFile := bufio.NewWriterSize(logFile, 8192)

	debugHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	  Level: slog.LevelDebug,
		ReplaceAttr: replaceAttr,
  })

  infoHandler := slog.NewJSONHandler(bufferedFile, &slog.HandlerOptions{
	  Level: slog.LevelInfo,
		ReplaceAttr: replaceAttr,
  })
 
  return slog.New(slog.NewMultiHandler(
	  debugHandler,
	  infoHandler,
  )), 
	bufferedFile.Flush, 
	nil
}
