package axtransport

import "context"

type DataHandlerFunc func(data []byte, ctx context.Context) ([]byte, error)
type DataReceiveFunc func(data []byte, ctx context.Context) error
