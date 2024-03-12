package axtransport

import "context"

type Transport struct {
	b                         *Builder
	tcpCtx, httpCtx           context.Context
	tcpCancelFn, httpCancelFn context.CancelFunc
}

func (t *Transport) Start() {
	t.StartHTTP()
	t.StartTCP()
}

func (t *Transport) StartTCP() {
	t.tcpCtx, t.tcpCancelFn = context.WithCancel(t.b.ctx)
}

func (t *Transport) StartHTTP() {
	t.httpCtx, t.httpCancelFn = context.WithCancel(t.b.ctx)
}

func (t *Transport) Stop() {
	t.StopHTTP()
	t.StopTCP()
}

func (t *Transport) StopHTTP() {
	if t.httpCancelFn != nil {
		t.httpCancelFn()
		t.httpCancelFn = nil
	}
}

func (t *Transport) StopTCP() {
	if t.tcpCancelFn != nil {
		t.tcpCancelFn()
		t.tcpCancelFn = nil
	}
}
