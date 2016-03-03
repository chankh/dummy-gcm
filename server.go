package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
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
	errc := make(chan error)
	var svc GcmService
	svc = gcmService{}
	svc = loggingMiddleware{svc, logger}

	go func() {
		errc <- interrupt()
	}()

	go func() {
		var (
			mux = http.NewServeMux()
		)

		gcm := makeGcmEndpoint(svc, *delay)
		mux.Handle("/gcm/send", httptransport.NewServer(
			root, gcm, decodeGcmRequest, encodeResponse))
		addr := fmt.Sprintf("%s:%d", *bind, *port)
		logger.Log("addr", addr)
		errc <- http.ListenAndServe(addr, mux)
	}()

	logger.Log("fatal", <-errc)
}

func decodeGcmRequest(r *http.Request) (interface{}, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
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
		req := request.(string)

		return svc.Send(req)
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

type loggingMiddleware struct {
	GcmService
	log.Logger
}

func (m loggingMiddleware) Send(s string) (output string, err error) {
	defer func(begin time.Time) {
		m.Logger.Log(
			"method", "send",
			"input", s,
			"output", output,
			"err", err,
			"took", time.Since(begin))
	}(time.Now())
	output, err = m.GcmService.Send(s)
	return
}
