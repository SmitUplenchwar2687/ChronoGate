package chronogatecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronoserver "github.com/SmitUplenchwar2687/Chrono/pkg/server"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
	"github.com/SmitUplenchwar2687/ChronoGate/internal/app"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var (
		addr        string
		algorithm   string
		rate        int
		window      string
		burst       int
		embedChrono bool
		chronoAddr  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run ChronoGate HTTP API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadServeConfig(cmd)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("addr") {
				cfg.Addr = strings.TrimSpace(addr)
			}
			if cmd.Flags().Changed("algorithm") {
				cfg.Algorithm = limiter.Algorithm(strings.TrimSpace(algorithm))
			}
			if cmd.Flags().Changed("rate") {
				cfg.Rate = rate
			}
			if cmd.Flags().Changed("burst") {
				cfg.Burst = burst
			}
			if cmd.Flags().Changed("window") {
				parsed, parseErr := time.ParseDuration(strings.TrimSpace(window))
				if parseErr != nil {
					return fmt.Errorf("parse --window: %w", parseErr)
				}
				cfg.Window = parsed
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			return runServe(cmd.Context(), cfg, embedChrono, chronoAddr, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "", "HTTP bind address (default from ADDR env or :8080)")
	cmd.Flags().StringVar(&algorithm, "algorithm", "", "rate limiting algorithm: token_bucket|sliding_window|fixed_window")
	cmd.Flags().IntVar(&rate, "rate", 0, "requests allowed per window")
	cmd.Flags().StringVar(&window, "window", "", "rate limit window duration")
	cmd.Flags().IntVar(&burst, "burst", 0, "token bucket burst size")
	cmd.Flags().BoolVar(&embedChrono, "embed-chrono", false, "start Chrono SDK server alongside ChronoGate")
	cmd.Flags().StringVar(&chronoAddr, "chrono-addr", ":9090", "embedded Chrono server address")

	return cmd
}

func loadServeConfig(cmd *cobra.Command) (app.Config, error) {
	cfg, err := app.LoadConfigFromEnv()
	if err != nil {
		return app.Config{}, fmt.Errorf("load config from env: %w", err)
	}
	return cfg, nil
}

func runServe(ctx context.Context, cfg app.Config, embedChrono bool, chronoAddr string, out io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	clk := chronoclock.NewRealClock()
	lim, err := app.NewLimiter(cfg, clk)
	if err != nil {
		return fmt.Errorf("create ChronoGate limiter: %w", err)
	}

	rec := chronorecorder.New(nil)
	store := chronostorage.NewMemoryStorage(clk)
	handler := app.NewHandler(lim, clk, rec, store)

	gateServer := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	errCh := make(chan error, 2)
	go func() {
		fmt.Fprintf(out, "ChronoGate listening on %s\n", cfg.Addr)
		if serveErr := gateServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	var embeddedChrono *chronoserver.Server
	if embedChrono {
		chronoClk := chronoclock.NewRealClock()
		chronoLim, limErr := app.NewLimiter(cfg, chronoClk)
		if limErr != nil {
			return fmt.Errorf("create embedded Chrono limiter: %w", limErr)
		}
		embeddedChrono = chronoserver.New(chronoAddr, chronoLim, chronoClk)
		go func() {
			fmt.Fprintf(out, "Embedded Chrono SDK server listening on %s\n", chronoAddr)
			if serveErr := embeddedChrono.Start(); serveErr != nil && !isClosedServerErr(serveErr) {
				errCh <- serveErr
			}
		}()
	}

	select {
	case serveErr := <-errCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if embeddedChrono != nil {
			_ = embeddedChrono.Shutdown(shutdownCtx)
		}
		_ = gateServer.Shutdown(shutdownCtx)
		return serveErr
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if embeddedChrono != nil {
		if err := embeddedChrono.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown embedded Chrono server: %w", err)
		}
	}
	if err := gateServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown ChronoGate server: %w", err)
	}

	fmt.Fprintln(out, "ChronoGate stopped")
	return nil
}

func isClosedServerErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, http.ErrServerClosed) {
		return true
	}
	return strings.Contains(err.Error(), "Server closed")
}
