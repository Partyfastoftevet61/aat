// Command aat-sandbox serves the offline shop API used by examples/shop and
// extracts that example project into a directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	aat "github.com/gburgyan/aat"
	"github.com/gburgyan/aat/internal/sandbox/shop"
	"github.com/gburgyan/aat/internal/sandbox/shopgrpc"
	"github.com/gburgyan/aat/internal/version"
)

// exitError wraps an error with a specific process exit code.
type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func (e *exitError) Unwrap() error { return e.Err }

var rootCmd = &cobra.Command{
	Use:   "aat-sandbox",
	Short: "Offline demo API for AAT: an e-commerce sandbox with regions, auth, and chaos",
	Long: `aat-sandbox serves a small, deterministic e-commerce API on your machine so the
examples/shop AAT project runs with no signup and no network: two regions with
their own currency and tax rules, an OAuth2 token endpoint, a separately
authenticated payments listener, an order state machine, simulated latency,
and two chaos hooks for retry demos.

The payments API is served twice, over HTTP and over gRPC, against the same
orders, so examples/grpc-payments can charge over gRPC an order it opened
over HTTP.

  aat-sandbox init shop && cd shop     # extract the example project
  aat-sandbox serve &                  # shop API :8765, payments :8766, payments over gRPC :8767
  aat run plan full-lifecycle`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Effective(), version.GitCommit, version.BuildDate),
}

// serveArgs holds the resolved flags for `aat-sandbox serve`.
type serveArgs struct {
	Host     string
	APIPort  int
	PayPort  int
	GRPCPort int
	Latency  float64
	Seed     int64
	NoAuth   bool
	Quiet    bool
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the shop API (:8765), payments (:8766), and gRPC payments (:8767) listeners",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		f := cmd.Flags()
		var a serveArgs
		a.Host, _ = f.GetString("host")
		a.APIPort, _ = f.GetInt("api-port")
		a.PayPort, _ = f.GetInt("pay-port")
		a.GRPCPort, _ = f.GetInt("grpc-port")
		a.Latency, _ = f.GetFloat64("latency")
		a.Seed, _ = f.GetInt64("seed")
		a.NoAuth, _ = f.GetBool("no-auth")
		a.Quiet, _ = f.GetBool("quiet")

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return serveCommand(ctx, a, os.Stderr, nil)
	},
}

// serveCommand runs every listener until ctx is cancelled. onReady, when set,
// receives the bound addresses once they all accept connections.
func serveCommand(ctx context.Context, a serveArgs, out io.Writer, onReady func(apiAddr, payAddr, grpcAddr string)) error {
	srv := shop.New(shop.Options{Latency: a.Latency, Seed: a.Seed, NoAuth: a.NoAuth})

	apiLn, err := net.Listen("tcp", net.JoinHostPort(a.Host, strconv.Itoa(a.APIPort)))
	if err != nil {
		return fmt.Errorf("shop API listener: %w", err)
	}
	payLn, err := net.Listen("tcp", net.JoinHostPort(a.Host, strconv.Itoa(a.PayPort)))
	if err != nil {
		_ = apiLn.Close()
		return fmt.Errorf("payments listener: %w", err)
	}

	grpcLn, err := net.Listen("tcp", net.JoinHostPort(a.Host, strconv.Itoa(a.GRPCPort)))
	if err != nil {
		_ = apiLn.Close()
		_ = payLn.Close()
		return fmt.Errorf("gRPC payments listener: %w", err)
	}

	// The gRPC service is a façade over the payments handler, served from the
	// same descriptor set examples/grpc-payments reads, so the two cannot
	// describe different APIs.
	descriptors, err := aat.PaymentsDescriptorSet()
	if err != nil {
		_ = apiLn.Close()
		_ = payLn.Close()
		_ = grpcLn.Close()
		return fmt.Errorf("loading the payments descriptor set: %w", err)
	}
	payGRPC, err := shopgrpc.New(descriptors, srv.PaymentsHandler(), shop.DefaultRegion)
	if err != nil {
		_ = apiLn.Close()
		_ = payLn.Close()
		_ = grpcLn.Close()
		return err
	}
	grpcSrv := grpc.NewServer()
	if err := payGRPC.Register(grpcSrv); err != nil {
		_ = apiLn.Close()
		_ = payLn.Close()
		_ = grpcLn.Close()
		return err
	}

	apiSrv := &http.Server{Handler: srv.APIHandler(), ReadHeaderTimeout: 10 * time.Second}
	paySrv := &http.Server{Handler: srv.PaymentsHandler(), ReadHeaderTimeout: 10 * time.Second}

	errCh := make(chan error, 3)
	go func() { errCh <- apiSrv.Serve(apiLn) }()
	go func() { errCh <- paySrv.Serve(payLn) }()
	go func() { errCh <- grpcSrv.Serve(grpcLn) }()

	apiAddr, payAddr, grpcAddr := displayAddr(apiLn.Addr()), displayAddr(payLn.Addr()), displayAddr(grpcLn.Addr())
	if !a.Quiet {
		srv.Banner(out, apiAddr, payAddr, shopgrpc.BannerLine(grpcAddr, shop.DemoAPIKey))
	}
	if onReady != nil {
		onReady(apiAddr, payAddr, grpcAddr)
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = apiSrv.Shutdown(shutdownCtx)
	_ = paySrv.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
	if !a.Quiet {
		_, _ = fmt.Fprintln(out, "aat-sandbox: stopped")
	}
	return nil
}

// displayAddr renders a bound address as host:port, substituting localhost
// for the unspecified address so the banner shows a URL that works.
func displayAddr(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return addr.String()
	}
	host := tcp.IP.String()
	if tcp.IP == nil || tcp.IP.IsUnspecified() {
		host = "localhost"
	}
	return net.JoinHostPort(host, strconv.Itoa(tcp.Port))
}

// initArgs holds the resolved flags for `aat-sandbox init`.
type initArgs struct {
	Dir   string
	Force bool
	// Example names the embedded project to extract; empty means shop.
	Example string
}

// examples are the projects init can extract, each with the plan its next
// steps suggest running first.
var examples = map[string]struct {
	fs        func() (fs.FS, error)
	firstPlan string
}{
	"shop":          {aat.ShopExampleFS, "full-lifecycle"},
	"grpc-payments": {aat.GRPCPaymentsExampleFS, "charge-and-refund"},
}

var initCmd = &cobra.Command{
	Use:   "init <dir>",
	Short: "Extract an example AAT project into a directory: examples/shop, or examples/grpc-payments with --example",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		example, _ := cmd.Flags().GetString("example")
		return initCommand(initArgs{Dir: args[0], Force: force, Example: example}, cmd.OutOrStdout())
	},
}

// initCommand writes the embedded example project into a.Dir. A non-empty
// directory is refused unless Force is set.
func initCommand(a initArgs, out io.Writer) error {
	if a.Dir == "" {
		return &exitError{Code: 2, Err: errors.New("init: target directory is required")}
	}
	if a.Example == "" {
		a.Example = "shop"
	}
	example, known := examples[a.Example]
	if !known {
		return &exitError{Code: 2, Err: fmt.Errorf("init: unknown example %q: the examples are grpc-payments and shop", a.Example)}
	}
	if entries, err := os.ReadDir(a.Dir); err == nil {
		if len(entries) > 0 && !a.Force {
			return &exitError{Code: 1, Err: fmt.Errorf("directory %s is not empty; use --force to overwrite", a.Dir)}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("init: %w", err)
	}

	src, err := example.fs()
	if err != nil {
		return fmt.Errorf("init: embedded example: %w", err)
	}
	count := 0
	err = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(a.Dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil { //nolint:gosec // example project files, world-readable by design
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Extracted %d files to %s\n\nNext steps:\n  cd %s\n  aat-sandbox serve &\n  aat run plan %s\n", count, a.Dir, a.Dir, example.firstPlan)
	return nil
}

func init() {
	f := serveCmd.Flags()
	f.String("host", "127.0.0.1", "interface to bind; use 0.0.0.0 to accept connections from other machines or containers")
	f.Int("api-port", 8765, "port for the shop API listener")
	f.Int("pay-port", 8766, "port for the payments API listener")
	f.Int("grpc-port", 8767, "port for the gRPC payments listener (shop.v1.Payments)")
	f.Float64("latency", 1.0, "scale factor for simulated latency (0 disables; shipOrder 600 ms, paymentCharge 350 ms)")
	f.Int64("seed", 1, "seed for tokens and tracking numbers (0 = time-based)")
	f.Bool("no-auth", false, "disable the bearer token and API key checks")
	f.Bool("quiet", false, "suppress the startup banner")

	initCmd.Flags().Bool("force", false, "write into a non-empty directory, overwriting files")
	initCmd.Flags().String("example", "shop", "the project to extract: shop, or grpc-payments (one plan across HTTP and gRPC)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
}

func main() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			if exitErr.Code != 0 && exitErr.Err != nil {
				fmt.Fprintf(os.Stderr, "aat-sandbox: %s\n", exitErr.Err)
			}
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "aat-sandbox: %s\n", err)
		os.Exit(1)
	}
}
