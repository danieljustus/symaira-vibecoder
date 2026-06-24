package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/server/tlsutil"
)

func pairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pair",
		Short: "Generate a QR pairing code for a remote device",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}
			if cfg.Server.Access == "loopback" || cfg.Server.Access == "" {
				fmt.Fprintln(os.Stderr, "warning: server.access is loopback; pairing is designed for lan/relay mode")
			}

			hostname, _ := os.Hostname()
			pair, err := tlsutil.EnsureCert(config.DataDir(), hostname)
			if err != nil {
				return fmt.Errorf("tls: %w", err)
			}

			code := generatePairCode()
			payload := buildPairPayload(cfg, pair.Fingerprint, code)

			fmt.Println()
			fmt.Println("  ┌──────────────────────────────────────────────────────┐")
			fmt.Println("  │  Pair with iPhone / Remote Device                    │")
			fmt.Println("  │                                                      │")
			fmt.Println("  │  Open the app and scan or enter this pairing code:   │")
			fmt.Println("  └──────────────────────────────────────────────────────┘")
			fmt.Println()
			fmt.Printf("  Pairing code:  %s\n", code)
			fmt.Printf("  Expires in:    120 seconds\n")
			fmt.Printf("  TLS cert SHA:  %s\n", pair.Fingerprint[:16])
			fmt.Println()
			fmt.Println("  QR Payload (manual entry fallback):")
			fmt.Printf("  %s\n", payload)
			fmt.Println()
			fmt.Println("  ── ASCII QR (approximate) ──")
			fmt.Println()
			printQR(payload)
			fmt.Println()
			fmt.Println("  Start the server:  symvibe serve --access lan")
			fmt.Println()
			return nil
		},
	}
}

func buildPairPayload(cfg *config.Config, fp, code string) string {
	host := cfg.Server.Host
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	name, _ := os.Hostname()
	if name == "" {
		name = "symvibe"
	}
	return fmt.Sprintf("symvibe://pair?n=%s&p=%d&h=%s&fp=%s&c=%s",
		name, cfg.Server.Port, host, fp, code)
}

func generatePairCode() string {
	const charset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
	} else {
		for i := range b {
			b[i] = charset[int(b[i])%len(charset)]
		}
	}
	return string(b)
}

func printQR(text string) {
	h := hex.EncodeToString([]byte(text))
	lines := make([]string, 0)
	lines = append(lines, "\x1b[0;37m  ╔══════════════════════════════════════╗\x1b[0m")
	lines = append(lines, "\x1b[0;37m  ║\x1b[0m                                      \x1b[0;37m║\x1b[0m")

	for i := 0; i < len(h); i += 32 {
		end := i + 32
		if end > len(h) {
			end = len(h)
		}
		chunk := h[i:end]
		visual := hexToBlocks(chunk)
		lines = append(lines, fmt.Sprintf("\x1b[0;37m  ║\x1b[0m  %s  \x1b[0;37m║\x1b[0m", visual))
	}

	lines = append(lines, "\x1b[0;37m  ║\x1b[0m                                      \x1b[0;37m║\x1b[0m")
	lines = append(lines, "\x1b[0;37m  ╚══════════════════════════════════════╝\x1b[0m")

	for _, l := range lines {
		fmt.Println(l)
	}
}

func hexToBlocks(h string) string {
	var b strings.Builder
	for _, c := range h {
		switch c {
		case '0', '1', '2', '3':
			b.WriteString("░░")
		case '4', '5', '6', '7':
			b.WriteString("▒▒")
		case '8', '9', 'a', 'b':
			b.WriteString("▓▓")
		default:
			b.WriteString("██")
		}
	}
	return b.String()
}
