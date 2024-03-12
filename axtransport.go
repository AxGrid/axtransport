package axtransport

type Transport struct {
	b    *Builder
	tcp  *AxTcp
	http *AxHttp
}

func (t *Transport) Start() error {
	t.StartHTTP()
	return t.StartTCP()
}

func (t *Transport) StartTCP() error {
	if t.tcp != nil {
		err := t.tcp.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Transport) StartHTTP() {
	if t.http != nil {
		t.http.Start()
	}
}

func (t *Transport) Stop() {
	t.StopHTTP()
	t.StopTCP()
}

func (t *Transport) StopHTTP() {
	if t.http != nil {
		t.http.Stop()
	}
}

func (t *Transport) StopTCP() {
	if t.tcp != nil {
		t.tcp.Stop()
	}
}
