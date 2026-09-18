package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TLSFiles is a private certificate authority and the certificates it signed,
// written to a directory as the PEM files an environment's grpc.tls block
// names.
type TLSFiles struct {
	// Dir holds the files below, which are named relative to it.
	Dir string
	// CAFile is the authority's certificate: what a client trusts.
	CAFile string
	// ClientCertFile and ClientKeyFile are a client certificate, for a server
	// that asks for one.
	ClientCertFile string
	ClientKeyFile  string

	ca     *x509.Certificate
	server tls.Certificate
}

// ServerName is a name the server certificate carries besides 127.0.0.1, for
// a test that reaches the server by an address its certificate does not name.
const ServerName = "api.internal"

// WriteTLSFiles makes a certificate authority, a server certificate for
// 127.0.0.1 and ServerName, and a client certificate, and writes the PEM files
// a client needs into a temporary directory.
func WriteTLSFiles(t *testing.T) *TLSFiles {
	t.Helper()
	dir := t.TempDir()

	caKey := newKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "aat test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	must(t, err)
	ca, err := x509.ParseCertificate(caDER)
	must(t, err)

	files := &TLSFiles{
		Dir:            dir,
		CAFile:         "ca.pem",
		ClientCertFile: "client.pem",
		ClientKeyFile:  "client-key.pem",
		ca:             ca,
	}
	writePEM(t, filepath.Join(dir, files.CAFile), "CERTIFICATE", caDER)

	serverDER, serverKey := files.sign(t, caKey, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "aat test server"},
		DNSNames:     []string{ServerName},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	files.server = tls.Certificate{Certificate: [][]byte{serverDER}, PrivateKey: serverKey}

	clientDER, clientKey := files.sign(t, caKey, &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "aat test client"},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	writePEM(t, filepath.Join(dir, files.ClientCertFile), "CERTIFICATE", clientDER)
	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	must(t, err)
	writePEM(t, filepath.Join(dir, files.ClientKeyFile), "EC PRIVATE KEY", keyDER)
	return files
}

// Path returns one of the files as an absolute path.
func (f *TLSFiles) Path(name string) string { return filepath.Join(f.Dir, name) }

// ServerConfig returns the server's TLS settings. With requireClientCert the
// server refuses a client that presents no certificate the authority signed.
func (f *TLSFiles) ServerConfig(requireClientCert bool) *tls.Config {
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{f.server},
	}
	if requireClientCert {
		pool := x509.NewCertPool()
		pool.AddCert(f.ca)
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg
}

func (f *TLSFiles) sign(t *testing.T, caKey *ecdsa.PrivateKey, template *x509.Certificate) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key := newKey(t)
	template.NotBefore = time.Now().Add(-time.Hour)
	template.NotAfter = time.Now().Add(24 * time.Hour)
	template.KeyUsage = x509.KeyUsageDigitalSignature
	der, err := x509.CreateCertificate(rand.Reader, template, f.ca, &key.PublicKey, caKey)
	must(t, err)
	return der, key
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	return key
}

func writePEM(t *testing.T, path, kind string, der []byte) {
	t.Helper()
	must(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
