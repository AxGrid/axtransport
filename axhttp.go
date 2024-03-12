package axtransport

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"time"
)

type AxHttp struct {
	logger       zerolog.Logger
	parentCtx    context.Context
	ctx          context.Context
	cancelFn     context.CancelFunc
	timeout      time.Duration
	parentRouter chi.Router
	apiPath      string
	bind         string
	srv          *http.Server
	binProcessor *AxBinProcessor
	handlerFunc  DataHandlerFunc
}

func NewAxHttp(ctx context.Context, logger zerolog.Logger, bind string, apiPath string, handlerFunc DataHandlerFunc) *AxHttp {
	res := &AxHttp{
		logger:       logger,
		parentCtx:    ctx,
		timeout:      25 * time.Second,
		bind:         bind,
		parentRouter: chi.NewRouter(),
		apiPath:      apiPath,
		handlerFunc:  handlerFunc,
	}
	res.binProcessor = NewAxBinProcessor(logger).WithCompressionSize(1024)
	return res
}

func (a *AxHttp) WithAES(secretKey []byte) *AxHttp {
	a.binProcessor.WithAES(secretKey)
	return a
}

func (a *AxHttp) WithCompressionSize(size int) *AxHttp {
	a.binProcessor.WithCompressionSize(size)
	return a
}

func (a *AxHttp) WithTimeout(timeout time.Duration) *AxHttp {
	a.timeout = timeout
	return a
}

func (a *AxHttp) WithRouter(r chi.Router) *AxHttp {
	r.Post(a.apiPath, a.handler)
	a.parentRouter = r
	return a
}

func (a *AxHttp) Start() {
	a.logger.Info().Msgf("Starting HTTP server on %s", a.bind)
	if a.ctx != nil && a.ctx.Err() == nil {
		a.logger.Warn().Msg("HTTP server already started")
		return
	}
	a.ctx, a.cancelFn = context.WithCancel(a.parentCtx)
	a.srv = &http.Server{
		Addr:         a.bind,
		Handler:      a.parentRouter,
		ReadTimeout:  a.timeout,
		WriteTimeout: a.timeout,
	}
	go func() {
		if err := a.srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				a.logger.Info().Msg("HTTP server closed")
			} else {
				a.logger.Fatal().Err(err).Msg("HTTP server failed")
			}
		}
	}()
	go func() {
		<-a.ctx.Done()
		err := a.srv.Shutdown(a.ctx)
		if err != nil {
			a.logger.Error().Err(err).Msg("HTTP server failed to stop")
		}
	}()
}

func (a *AxHttp) Stop() {
	a.logger.Info().Msg("Stopping HTTP server")
	if err := a.ctx.Err(); err != nil {
		a.logger.Warn().Msg("HTTP server already stopped")
		return
	}
	a.cancelFn()
}

func (a *AxHttp) process([]byte) ([]byte, error) {
	return nil, nil
}

func (a *AxHttp) handler(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeHttpErr(w, http.StatusInternalServerError, err)
		return
	}
	defer r.Body.Close()
	data, err = a.binProcessor.Unmarshal(data)
	if err != nil {
		writeHttpErr(w, http.StatusBadRequest, err)
		return
	}
	data, err = a.handlerFunc(data, nil)
	if err != nil {
		writeHttpErr(w, http.StatusInternalServerError, err)
		return
	}
	data, err = a.process(data)
	if err != nil {
		writeHttpErr(w, http.StatusInternalServerError, err)
		return
	}
	data, err = a.binProcessor.Marshal(data)
	if err != nil {
		writeHttpErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

func writeHttpErr(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	errStackTrace := fmt.Sprintf("%+v", err)
	errString := fmt.Sprintf("(%d) %s\n%s", code, err.Error(), errStackTrace)
	_, _ = w.Write([]byte(errString))
}
