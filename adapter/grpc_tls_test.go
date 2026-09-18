package adapter

import (
	"context"
	"testing"

	"github.com/gburgyan/aat/internal/protoreg"
	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

// These tests send calls over TLS. A production gRPC API is reached by
// grpcs://, and building a tls.Config correctly says nothing about whether a
// handshake with it succeeds, so each setting an environment's grpc.tls block
// offers is exercised against a server that needs it.

// serveShopTLS runs the shop server behind TLS and returns its address, the
// descriptors, and the certificate files a client is configured with.
func serveShopTLS(t *testing.T, requireClientCert bool) (string, *protoreg.Registry, *testutil.TLSFiles) {
	t.Helper()
	files := testutil.WriteTLSFiles(t)
	addr, reg := serveShop(t, func(context.Context, *dynamicpb.Message) (proto.Message, error) {
		md, err := exec_reg(t).Method("shop.v1.Carts", "CreateCart")
		require.NoError(t, err)
		return cartReply(t, exec_reg(t), md.Output(), `{"cartId":"cart_tls"}`), nil
	}, grpc.Creds(credentials.NewTLS(files.ServerConfig(requireClientCert))))
	return addr, reg, files
}

func TestGRPCExecutor_TLS(t *testing.T) {
	addr, reg, files := serveShopTLS(t, false)
	ca := files.Path(files.CAFile)

	tests := []struct {
		name   string
		target string
		tls    TLSConfig
		// wantStatus is the gRPC status of a call that was answered.
		wantStatus string
		// wantErr marks a call whose handshake must fail. It is an error, not
		// an UNAVAILABLE response, because UNAVAILABLE is retried and a
		// certificate problem fails the same way every time. What crypto/x509
		// says differs by platform, so only the fact of it is asserted.
		wantErr bool
	}{
		{
			name:       "a private CA, and the name its certificate carries",
			target:     "grpcs://" + addr,
			tls:        TLSConfig{CAFile: ca, ServerName: testutil.ServerName},
			wantStatus: "OK",
		},
		{
			name:    "the system roots do not know a private CA",
			target:  "grpcs://" + addr,
			tls:     TLSConfig{ServerName: testutil.ServerName},
			wantErr: true,
		},
		{
			name:    "an address the certificate does not name",
			target:  "grpcs://" + addr,
			tls:     TLSConfig{CAFile: ca},
			wantErr: true,
		},
		{
			name:    "a serverName the certificate does not carry",
			target:  "grpcs://" + addr,
			tls:     TLSConfig{CAFile: ca, ServerName: "other.internal"},
			wantErr: true,
		},
		{
			name:       "insecureSkipVerify, for a sandbox",
			target:     "grpcs://" + addr,
			tls:        TLSConfig{Insecure: true},
			wantStatus: "OK",
		},
		{
			// No handshake is attempted, so there is no TLS error to report:
			// the server drops a client that does not speak TLS.
			name:       "plaintext to a TLS port",
			target:     "grpc://" + addr,
			tls:        TLSConfig{},
			wantStatus: "UNAVAILABLE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := shopExecutor(t, tt.target, reg, tt.tls)

			resp, err := exec.Execute(context.Background(), grpcRequest(`{}`))

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, err.Error(), "the TLS handshake failed: x509: ")
				assert.Contains(t, err.Error(), "grpc.tls", "the error says where the setting is")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp.GRPC)
			assert.Equal(t, tt.wantStatus, resp.GRPC.Name, "body: %s", resp.Body)
		})
	}
}

func TestGRPCExecutor_MutualTLS(t *testing.T) {
	addr, reg, files := serveShopTLS(t, true)
	ca := files.Path(files.CAFile)

	t.Run("a client certificate the server's CA signed", func(t *testing.T) {
		exec := shopExecutor(t, "grpcs://"+addr, reg, TLSConfig{
			CAFile:     ca,
			ServerName: testutil.ServerName,
			CertFile:   files.Path(files.ClientCertFile),
			KeyFile:    files.Path(files.ClientKeyFile),
		})

		resp, err := exec.Execute(context.Background(), grpcRequest(`{}`))

		require.NoError(t, err)
		assert.Equal(t, "OK", resp.GRPC.Name, "body: %s", resp.Body)
	})

	// Under TLS 1.3 a server refuses a missing client certificate after the
	// client's handshake has finished, so the client learns of it on its first
	// read: as the server's alert if that arrives first, and as a closed
	// connection if the close does. Which one is a race the client cannot win,
	// so the call is either the TLS error or UNAVAILABLE. It is never answered.
	t.Run("no client certificate is never answered", func(t *testing.T) {
		exec := shopExecutor(t, "grpcs://"+addr, reg, TLSConfig{CAFile: ca, ServerName: testutil.ServerName})

		resp, err := exec.Execute(context.Background(), grpcRequest(`{}`))

		if err != nil {
			assert.Contains(t, err.Error(), "the TLS handshake failed")
			assert.Contains(t, err.Error(), "certFile")
			return
		}
		require.NotNil(t, resp.GRPC)
		assert.Equal(t, "UNAVAILABLE", resp.GRPC.Name, "body: %s", resp.Body)
	})
}
