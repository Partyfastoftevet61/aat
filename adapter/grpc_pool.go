package adapter

import (
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnPool hands out gRPC connections and keeps them for the life of a run.
//
// An HTTP client pools its own connections; a gRPC one does not. Dialing per
// step would pay a TLS and HTTP/2 handshake every time and, on a long plan,
// run a process out of file descriptors. A ClientConn is safe for concurrent
// use and reconnects on its own, so one per target serves every step, parallel
// ones included.
type ConnPool struct {
	mu     sync.Mutex
	conns  map[string]*grpc.ClientConn
	closed bool
}

// NewConnPool returns an empty pool.
func NewConnPool() *ConnPool {
	return &ConnPool{conns: make(map[string]*grpc.ClientConn)}
}

// Get returns the connection for a target, dialing one the first time it is
// asked for.
//
// It uses grpc.NewClient, which does not connect: the first RPC does, under
// the deadline GRPCExecutor.Execute puts on the call. Dialing is therefore
// covered by aat's request timeout, and an unreachable host fails the step
// that needed it rather than the run that set it up.
func (p *ConnPool) Get(target string, secure bool, tlsCfg TLSConfig) (*grpc.ClientConn, error) {
	dialTarget, _, err := ParseGRPCTarget(target)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s|%t|%s", dialTarget, secure, tlsCfg.key())

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("connection pool is closed")
	}
	if conn, ok := p.conns[key]; ok {
		return conn, nil
	}

	creds := insecure.NewCredentials()
	if secure {
		cfg, err := tlsCfg.build()
		if err != nil {
			return nil, fmt.Errorf("securing the connection to %s: %w", target, err)
		}
		creds = credentials.NewTLS(cfg)
	}

	conn, err := grpc.NewClient(dialTarget,
		grpc.WithTransportCredentials(creds),
		// AAT retries steps itself, with its own categories, ceilings, and
		// pacing. Leaving gRPC's retry on as well would double every attempt
		// and miscount the requests a mutation made.
		grpc.WithDisableRetry(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", target, err)
	}
	p.conns[key] = conn
	return conn, nil
}

// Close closes every connection the pool holds. It is idempotent.
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	var errs []error
	for key, conn := range p.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing %s: %w", key, err))
		}
	}
	p.conns = nil
	return errors.Join(errs...)
}
