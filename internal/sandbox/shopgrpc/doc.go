// Package shopgrpc serves the shop sandbox's payments API over gRPC.
//
// It is a façade, not a second implementation: every call is turned into the
// HTTP request the payments handler already answers, and its response is
// turned back into a protobuf message. The two surfaces therefore cannot drift
// — the order state machine, the declined card, the gift-card balances, and
// the region's currency rules are the ones in internal/sandbox/shop, reached
// through the same code path.
//
// That is not how a production gRPC service is built. It is how this one
// should be: the sandbox exists to exercise AAT's client, and a façade keeps
// the behaviour a plan sees identical whichever protocol it used.
//
// Unlike internal/sandbox/shop, which is stdlib only, this package depends on
// google.golang.org/grpc and google.golang.org/protobuf. It has no generated
// code: the service is served dynamically from the descriptor set that
// examples/grpc-payments ships, which is the same artifact AAT reads.
package shopgrpc
