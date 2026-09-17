package engine

import (
	"path/filepath"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/config"
)

// GRPCTLS returns an environment's gRPC TLS settings in the form the adapter
// takes, resolving certificate paths against baseDir so an environment file
// names its certificates beside itself.
//
// config and adapter are both leaf packages, so neither can convert to the
// other; engine already bridges them for routing and does so here too.
func GRPCTLS(env *config.Environment, baseDir string) adapter.TLSConfig {
	if env == nil || env.GRPC == nil || env.GRPC.TLS == nil {
		return adapter.TLSConfig{}
	}
	t := env.GRPC.TLS
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, p)
	}
	return adapter.TLSConfig{
		CAFile:     resolve(t.CAFile),
		CertFile:   resolve(t.CertFile),
		KeyFile:    resolve(t.KeyFile),
		ServerName: t.ServerName,
		Insecure:   t.InsecureSkipVerify,
	}
}
