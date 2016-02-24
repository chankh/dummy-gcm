package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
)

func main() {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	var (
		port  = fs.Int("port", 8081, "Server port")
		bind  = fs.String("bind", "0.0.0.0", "Bind address")
		delay = fs.Int("delay", 0, "Simulate some delay (in milliseconds) before sending a response")
	)

	flag.Usage = fs.Usage // only show our flags
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}

	// package log
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC).With("caller", log.DefaultCaller)
		stdlog.SetFlags(0)                             // flags are handled by Go kit's logger
		stdlog.SetOutput(log.NewStdlibAdapter(logger)) // redirect anything using stdlib log to us
	}

	// Mechanical stuff
	root := context.Background()
	svc := gcmService{}
	errc := make(chan error)

	go func() {
		errc <- interrupt()
	}()

	go func() {
		var (
			transportLogger = log.NewContext(logger).With("transport", "debug")
			mux             = http.NewServeMux()
		)

		gcm := makeGcmEndpoint(svc, *delay)
		mux.Handle("/gcm/send", httptransport.NewServer(
			root, gcm, decodeGcmRequest, encodeResponse, httptransport.ServerErrorLogger(transportLogger)))
		addr := fmt.Sprintf("%s:%d", *bind, *port)
		transportLogger.Log("addr", addr)
		errc <- http.ListenAndServe(addr, mux)
	}()

	logger.Log("fatal", <-errc)
}

func decodeGcmRequest(r *http.Request) (interface{}, error) {
	return "regId", nil
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
	switch t := response.(type) {
	case string:
		_, err := w.Write([]byte(t))
		return err
	}

	return json.NewEncoder(w).Encode(response)
}

func interrupt() error {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	return fmt.Errorf("%s", <-c)
}

func makeGcmEndpoint(svc GcmService, delay int) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		if delay != 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		return svc.Send("regId")
	}
}

type GcmService interface {
	Send(string) (string, error)
}

type gcmService struct{}

func (gcmService) Send(s string) (string, error) {
	if s == "" {
		return "", ErrEmpty
	}
	return "id=0:1370674827295849", nil
}

// ErrEmpty is returned when an input string is empty.
var ErrEmpty = errors.New("empty registration id")
