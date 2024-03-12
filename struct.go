package axtransport

import "context"

type DataHandlerFunc func(data []byte, ctx context.Context) ([]byte, error)
