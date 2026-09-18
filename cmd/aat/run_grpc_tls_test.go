package main

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/internal/protoreg"
	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/dynamicpb"
)

// serveCartsTLS serves shop.v1.Carts/CreateCart behind TLS, optionally asking
// for a client certificate, and returns the address it listens on.
func serveCartsTLS(t *testing.T, protoset string, files *testutil.TLSFiles, requireClientCert bool) string {
	t.Helper()
	reg, err := protoreg.LoadDescriptorSets(protoset)
	require.NoError(t, err)
	md, err := reg.Method("shop.v1.Carts", "CreateCart")
	require.NoError(t, err)

	desc := &grpc.ServiceDesc{
		ServiceName: "shop.v1.Carts",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "CreateCart",
			Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				if err := dec(dynamicpb.NewMessage(md.Input())); err != nil {
					return nil, err
				}
				return reg.JSONToMessage(md.Output(), []byte(`{"cartId":"cart_tls"}`))
			},
		}},
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(files.ServerConfig(requireClientCert))))
	srv.RegisterService(desc, struct{}{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

// TestRunCommand_GRPCOverTLS runs a plan against a server that needs a private
// CA and a client certificate, configured the way a user configures them: a
// grpcs:// target and a grpc.tls block whose paths are beside env.yaml. It is
// the whole path from the environment file to the handshake.
func TestRunCommand_GRPCOverTLS(t *testing.T) {
	dir := grpcProject(t, true, false, "grpc", "shop.v1.Carts/CreateCart")
	files := testutil.WriteTLSFiles(t)
	addr := serveCartsTLS(t, filepath.Join(dir, "shop.protoset"), files, true)

	// The certificates go beside env.yaml, where grpc.tls paths resolve.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "certs"), 0o755))
	for _, name := range []string{files.CAFile, files.ClientCertFile, files.ClientKeyFile} {
		data, err := os.ReadFile(files.Path(name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", name), data, 0o600))
	}
	planPath := filepath.Join(dir, "plan.yaml")
	require.NoError(t, os.WriteFile(planPath, []byte(`execution:
  steps:
    - id: cart
      node: createCart
      assertions:
        mechanical:
          - type: status
            expect: OK
`), 0o644))

	run := func(t *testing.T, tlsBlock string) *runResult {
		t.Helper()
		envPath := filepath.Join(dir, "env.yaml")
		require.NoError(t, os.WriteFile(envPath, []byte("environment: tls\napiBaseUrl: grpcs://"+addr+"\n"+tlsBlock), 0o644))
		return runCommand(context.Background(), &runArgs{
			PlanPath:        planPath,
			EnvPath:         envPath,
			GraphPath:       filepath.Join(dir, "graph.yaml"),
			TemplatesPath:   filepath.Join(dir, "templates"),
			ProtoPaths:      []string{filepath.Join(dir, "shop.protoset")},
			OutputDir:       filepath.Join(dir, "runs"),
			NoAutoOverrides: true,
		}, io.Discard, TerminalInfo{})
	}

	t.Run("a private CA and a client certificate", func(t *testing.T) {
		res := run(t, `grpc:
  tls:
    caFile: certs/ca.pem
    certFile: certs/client.pem
    keyFile: certs/client-key.pem
    serverName: `+testutil.ServerName+`
`)
		require.NoError(t, res.err)
	})

	t.Run("without them the step says what failed, and where to look", func(t *testing.T) {
		res := run(t, "")
		require.Error(t, res.err)
		assert.Contains(t, res.err.Error(), "the TLS handshake failed")
		assert.Contains(t, res.err.Error(), "grpc.tls")
	})
}
