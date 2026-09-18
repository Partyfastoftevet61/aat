package adapter

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// Target schemes an environment writes to route a node over gRPC. Everything
// else is HTTP, so an existing project's URLs keep their meaning.
const (
	grpcScheme  = "grpc://"
	grpcsScheme = "grpcs://"
)

// TLSConfig describes how a gRPC executor secures its connection. The zero
// value means the scheme decides: grpcs:// uses the system roots, grpc:// is
// plaintext.
type TLSConfig struct {
	// CAFile is a PEM bundle to verify the server against, in place of the
	// system roots.
	CAFile string
	// CertFile and KeyFile are a client certificate, for a server that asks
	// for one.
	CertFile string
	KeyFile  string
	// ServerName overrides the name verified against the certificate, for a
	// host reached by an address its certificate does not name.
	ServerName string
	// Insecure skips verification. It exists for a sandbox with a self-signed
	// certificate and says so wherever it is written.
	Insecure bool
}

// IsGRPCTarget reports whether a target names a gRPC service.
func IsGRPCTarget(target string) bool {
	return strings.HasPrefix(target, grpcScheme) || strings.HasPrefix(target, grpcsScheme)
}

// ParseGRPCTarget splits a gRPC target into what grpc.NewClient dials and
// whether the connection is secured.
//
// The scheme is stripped, because grpc.NewClient reads a scheme as the name of
// a resolver: passing "grpc://host:9090" through fails with "unknown
// resolver". A target that already names a resolver, such as "dns:///host", is
// left alone.
func ParseGRPCTarget(target string) (dialTarget string, secure bool, err error) {
	switch {
	case strings.HasPrefix(target, grpcsScheme):
		dialTarget, secure = strings.TrimPrefix(target, grpcsScheme), true
	case strings.HasPrefix(target, grpcScheme):
		dialTarget, secure = strings.TrimPrefix(target, grpcScheme), false
	default:
		return "", false, fmt.Errorf("target %q is not a gRPC target: it must start with %q or %q", target, grpcScheme, grpcsScheme)
	}

	// A target that names a resolver is grpc-go's to read, path and all.
	if resolverTarget.MatchString(dialTarget) {
		return dialTarget, secure, nil
	}

	dialTarget = strings.TrimSuffix(dialTarget, "/")
	if dialTarget == "" {
		return "", false, fmt.Errorf("gRPC target %q names no host", target)
	}
	// A path would be a service or method, which the template names instead.
	if strings.Contains(dialTarget, "/") {
		return "", false, fmt.Errorf("gRPC target %q has a path: a target is a host and port, and the template names the method", target)
	}

	// grpc-go dials port 443 when a target names none, whatever the scheme
	// said. That is right for grpcs:// and is written out here; for grpc:// it
	// would send plaintext to a TLS port and fail as UNAVAILABLE, or hang, with
	// nothing to say the port was the problem.
	if _, _, splitErr := net.SplitHostPort(dialTarget); splitErr != nil {
		if !secure {
			return "", false, fmt.Errorf("gRPC target %q names no port: a plaintext service has no default one, so write %s%s:PORT", target, grpcScheme, dialTarget)
		}
		dialTarget = net.JoinHostPort(strings.Trim(dialTarget, "[]"), "443")
	}
	return dialTarget, secure, nil
}

// resolverTarget matches a dial target that names a grpc-go resolver, such as
// "dns:///host:443" or "unix:///run/api.sock".
var resolverTarget = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:(/|[^0-9/])`)

// buildTLS turns a TLSConfig into the settings a secure connection dials with.
func (c TLSConfig) build() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.Insecure, //nolint:gosec // opt-in, for a sandbox with a self-signed certificate
	}

	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA bundle: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("reading CA bundle %s: no certificates found", c.CAFile)
		}
		cfg.RootCAs = roots
	}

	switch {
	case c.CertFile != "" && c.KeyFile != "":
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("reading client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	case c.CertFile != "" || c.KeyFile != "":
		return nil, fmt.Errorf("a client certificate needs both cert and key")
	}

	return cfg, nil
}

// key identifies a TLS configuration, so connections are pooled per setting
// rather than per target alone.
func (c TLSConfig) key() string {
	return strings.Join([]string{c.CAFile, c.CertFile, c.KeyFile, c.ServerName, fmt.Sprint(c.Insecure)}, "\x00")
}
