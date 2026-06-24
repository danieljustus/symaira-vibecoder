package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/browser"
	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/devices"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
	"github.com/danieljustus/symaira-vibecoder/internal/server"
	"github.com/danieljustus/symaira-vibecoder/internal/server/tlsutil"
	"github.com/danieljustus/symaira-vibecoder/web"
)

func serveCmd() *cobra.Command {
	var host string
	var port int
	var dir string
	var noOpen bool
	var access string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Baukasten board on localhost",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}
			if host != "" {
				cfg.Server.Host = host
			}
			if cmd.Flags().Changed("port") {
				cfg.Server.Port = port
			}
			if dir != "" {
				cfg.Runner.WorkingDir = dir
			}
			if noOpen {
				cfg.Server.OpenBrowser = false
			}
			if access != "" {
				cfg.Server.Access = access
			}
			cfg.Server.Host = deriveBindHost(cfg)
			if err := cfg.Validate(); err != nil {
				return err
			}

			res := config.NewResolver(cfg)
			run := runner.NewOpenCodeRunner(cfg.Runner.OpencodeBin, cfg.Runner.RequestTimeout.Std())
			bus := engine.NewBus()
			eng := engine.New(cfg, res, run, bus)
			srv := server.New(cfg, eng, web.DistFS())
			if cfg.Auth.Enabled {
				reg, err := devices.Open()
				if err != nil {
					slog.Warn("device registry unavailable", "err", err)
				} else {
					srv.SetDevices(reg)
					srv.SetTokenStore(reg)
				}
			}

			addr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", addr, err)
			}

			httpSrv := &http.Server{
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				<-ctx.Done()
				eng.Cancel()
				sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(sh)
			}()

			useTLS := cfg.Server.Access == "lan" || cfg.Server.Access == "relay"
			scheme := "http"
			if useTLS {
				hostname, _, _ := net.SplitHostPort(ln.Addr().String())
				pair, err := tlsutil.EnsureCert(config.DataDir(), hostname)
				if err != nil {
					return fmt.Errorf("tls: %w", err)
				}
				slog.Info("TLS cert ready", "fp", pair.Fingerprint[:16], "cert", pair.CertPath)
				scheme = "https"
				url := scheme + "://" + ln.Addr().String()
				fmt.Printf("\n  symvibe board → %s\n  (Ctrl-C to stop)\n\n", url)
				if cfg.Server.OpenBrowser {
					go func() { _ = browser.Open(url) }()
				}
				if ok, info := run.Available(ctx); ok {
					slog.Info("opencode backend ready", "version", info.Version, "path", info.Path)
				} else {
					slog.Warn("opencode not found — board runs read-only (Run disabled)", "detail", info.Detail)
				}
				if err := httpSrv.ServeTLS(ln, pair.CertPath, pair.KeyPath); err != nil && err != http.ErrServerClosed {
					return err
				}
			} else {
				url := scheme + "://" + ln.Addr().String()
				fmt.Printf("\n  symvibe board → %s\n  (Ctrl-C to stop)\n\n", url)
				if cfg.Server.OpenBrowser {
					go func() { _ = browser.Open(url) }()
				}
				if ok, info := run.Available(ctx); ok {
					slog.Info("opencode backend ready", "version", info.Version, "path", info.Path)
				} else {
					slog.Warn("opencode not found — board runs read-only (Run disabled)", "detail", info.Detail)
				}
				if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
					return err
				}
			}

			slog.Info("symvibe stopped")
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "bind host (default 127.0.0.1 from config)")
	cmd.Flags().IntVar(&port, "port", 0, "bind port (default 4317 from config; pass 0 for a random free port)")
	cmd.Flags().StringVar(&dir, "dir", "", "working directory the cycle operates on (default: current dir)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser")
	cmd.Flags().StringVar(&access, "access", "", "network access mode: loopback (default), lan, relay")
	return cmd
}

func deriveBindHost(cfg *config.Config) string {
	switch cfg.Server.Access {
	case "lan":
		if cfg.Server.Host == "127.0.0.1" || cfg.Server.Host == "" {
			return "0.0.0.0"
		}
		return cfg.Server.Host
	case "relay":
		return "0.0.0.0"
	default:
		if cfg.Server.Host == "" {
			return "127.0.0.1"
		}
		return cfg.Server.Host
	}
}
