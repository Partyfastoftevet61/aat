package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aat "github.com/gburgyan/aat"
)

func TestInitCommand_ExtractsEmbeddedFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shop")
	var out bytes.Buffer
	require.NoError(t, initCommand(initArgs{Dir: dir}, &out))
	assert.Contains(t, out.String(), "Extracted")
	assert.Contains(t, out.String(), "aat-sandbox serve")

	src, err := aat.ShopExampleFS()
	require.NoError(t, err)
	count := 0
	err = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		want, err := fs.ReadFile(src, p)
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		require.NoError(t, err, p)
		assert.Equal(t, want, got, p)
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Greater(t, count, 0, "the embedded example must contain files")
	_, err = os.Stat(filepath.Join(dir, "openapi.yaml"))
	assert.NoError(t, err, "openapi.yaml is part of the example")
}

func TestInitCommand_RefusesNonEmptyUnlessForced(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644))

	err := initCommand(initArgs{Dir: dir}, &bytes.Buffer{})
	var exitErr *exitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.Code)
	assert.Contains(t, err.Error(), "--force")
	_, statErr := os.Stat(filepath.Join(dir, "openapi.yaml"))
	assert.True(t, os.IsNotExist(statErr), "nothing is written when refused")

	require.NoError(t, initCommand(initArgs{Dir: dir, Force: true}, &bytes.Buffer{}))
	_, statErr = os.Stat(filepath.Join(dir, "openapi.yaml"))
	assert.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(dir, "keep.txt"))
	assert.NoError(t, statErr, "existing files are kept")
}

func TestServeCommand_HealthAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan [3]string, 1)
	errCh := make(chan error, 1)
	var out bytes.Buffer
	go func() {
		errCh <- serveCommand(ctx, serveArgs{Host: "127.0.0.1", Latency: 0, Seed: 1}, &out,
			func(api, pay, grpcAddr string) { ready <- [3]string{api, pay, grpcAddr} })
	}()

	var addrs [3]string
	select {
	case addrs = <-ready:
	case err := <-errCh:
		t.Fatalf("serve exited early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not become ready")
	}
	for _, addr := range addrs[:2] {
		res, err := http.Get("http://" + addr + "/healthz")
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusOK, res.StatusCode, addr)
	}
	// The gRPC listener answers no HTTP route, so readiness is a connection.
	conn, err := net.DialTimeout("tcp", addrs[2], 2*time.Second)
	require.NoError(t, err, "gRPC payments listener at %s", addrs[2])
	_ = conn.Close()

	cancel()
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not shut down")
	}
	assert.Contains(t, out.String(), "aat-sandbox: shop API")
	assert.Contains(t, out.String(), "aat-sandbox: stopped")
}

func TestServeCommand_PortInUse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan [3]string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveCommand(ctx, serveArgs{Host: "127.0.0.1", Quiet: true, Seed: 1}, &bytes.Buffer{},
			func(api, pay, grpcAddr string) { ready <- [3]string{api, pay, grpcAddr} })
	}()
	var addrs [3]string
	select {
	case addrs = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("first server not ready")
	}
	_, portStr, err := splitHostPort(addrs[0])
	require.NoError(t, err)

	err = serveCommand(context.Background(), serveArgs{Host: "127.0.0.1", APIPort: portStr, Quiet: true, Seed: 1}, &bytes.Buffer{}, nil)
	require.Error(t, err)
	assert.False(t, errors.Is(err, http.ErrServerClosed))
	assert.Contains(t, err.Error(), "shop API listener")
}

func splitHostPort(addr string) (string, int, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	n, err := strconv.Atoi(port)
	return host, n, err
}
