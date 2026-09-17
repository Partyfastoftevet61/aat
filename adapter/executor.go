package adapter

import "context"

// Protocol names the wire protocol an executor speaks. A template declares the
// same names in its protocol key.
const (
	ProtocolHTTP = "http"
)

// Executor sends an adapter-built request and returns the response. The engine
// holds executors only through this interface, so a router can mix protocols:
// one node's calls go over HTTP while another's go somewhere else entirely.
//
// Implementations are safe for concurrent use; the engine sends a batch's steps
// from several goroutines.
type Executor interface {
	// Execute sends req and returns the response. It respects the context for
	// cancellation and timeouts.
	Execute(ctx context.Context, req *Request) (*Response, error)

	// Protocol names the wire protocol, matching a template's protocol key.
	Protocol() string

	// Target names where the executor sends requests: a base URL for HTTP.
	// Archives and diagnostics report it as the step's actual host.
	Target() string

	// Close releases whatever the executor holds open. It is idempotent, and a
	// closed executor is not reused.
	Close() error
}
