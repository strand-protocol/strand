// Package server provides the StrandAPI server SDK. It listens for incoming
// StrandAPI frames, dispatches them to registered handlers, and manages
// connection lifecycles.
package server

import (
	"context"

	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
)

// Handler is implemented by types that handle non-streaming inference
// requests. A handler receives a decoded InferenceRequest and returns a
// complete InferenceResponse or an error.
type Handler interface {
	HandleInference(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error)
}

// TokenSender is provided to a StreamHandler so it can emit tokens one at a
// time. Each call to Send transmits a single TokenStreamChunk to the client.
type TokenSender interface {
	Send(chunk *protocol.TokenStreamChunk) error
}

// StreamHandler is implemented by types that handle streaming inference
// requests. The handler receives the request and a TokenSender. It should
// call sender.Send for each generated token and return nil on success.
type StreamHandler interface {
	HandleTokenStream(ctx context.Context, req *protocol.InferenceRequest, sender TokenSender) error
}

// HandlerFunc is an adapter to allow use of ordinary functions as Handlers.
type HandlerFunc func(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error)

// HandleInference calls f(ctx, req).
func (f HandlerFunc) HandleInference(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	return f(ctx, req)
}
