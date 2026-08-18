package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"slices"

	"boot.dev/linko/internal/linkoerr"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	pkgerr "github.com/pkg/errors"
	"gopkg.in/natefinch/lumberjack.v2"
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

var sensitiveKeys = []string{
	"password", 
	"key", 
	"apikey", 
	"secret", 
	"pin", 
	"creditcardno",
	"user",
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if slices.Contains(sensitiveKeys, a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}

	if a.Value.Kind() == slog.KindString {
		u, err := url.Parse(a.Value.String())
		if err != nil || u.User == nil {
			return  a
		}

		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), "[REDACTED]")
			a.Value = slog.StringValue(u.String())
		}
	}

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

func initializeLogger() (*slog.Logger, closeFunc, error) {
	filename := os.Getenv("LINKO_LOG_FILE")
	if filename == "" {
		filename = "linko.access.log"
	}
  logger := &lumberjack.Logger{
	  Filename:   filename,
	  MaxSize:    1,
	  MaxAge:     28,
	  MaxBackups: 10,
	  LocalTime:  false,
	  Compress:   true,
  }
	
	debugHandler := slog.Handler(tint.NewTextHandler(os.Stderr, &tint.Options{
	  Level: slog.LevelDebug,
		ReplaceAttr: replaceAttr,
		NoColor:     !(isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())),
  }))

  infoHandler := slog.NewJSONHandler(logger, &slog.HandlerOptions{
	  Level: slog.LevelInfo,
		ReplaceAttr: replaceAttr,
  })
 
  return slog.New(slog.NewMultiHandler(
	  debugHandler,
	  infoHandler,
  )), 
	logger.Close,
	nil
}
