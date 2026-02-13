package chronogatecli

import (
	"fmt"
	"strings"
	"time"

	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	"github.com/SmitUplenchwar2687/ChronoGate/internal/app"
	"github.com/spf13/cobra"
)

func newReplayCmd() *cobra.Command {
	var (
		file       string
		algorithm  string
		rate       int
		window     string
		burst      int
		speed      float64
		keys       string
		endpoints  string
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay recorded traffic through Chrono limiter algorithms",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			algoValue := cfg.Algorithm
			if cmd.Flags().Changed("algorithm") {
				algoValue = limiter.Algorithm(strings.TrimSpace(algorithm))
			}
			rateValue := cfg.Rate
			if cmd.Flags().Changed("rate") {
				rateValue = rate
			}
			burstValue := cfg.Burst
			if cmd.Flags().Changed("burst") {
				burstValue = burst
			}
			windowValue := cfg.Window
			if cmd.Flags().Changed("window") {
				parsed, parseErr := time.ParseDuration(strings.TrimSpace(window))
				if parseErr != nil {
					return fmt.Errorf("parse --window: %w", parseErr)
				}
				windowValue = parsed
			}

			tmp := app.Config{
				Algorithm:      algoValue,
				Rate:           rateValue,
				Window:         windowValue,
				Burst:          burstValue,
				Addr:           cfg.Addr,
				StorageBackend: cfg.StorageBackend,
			}
			if err := tmp.Validate(); err != nil {
				return err
			}

			_, err = app.RunReplay(cmd.Context(), app.ReplayOptions{
				File:      strings.TrimSpace(file),
				Algorithm: algoValue,
				Rate:      rateValue,
				Window:    windowValue,
				Burst:     burstValue,
				Speed:     speed,
				Keys:      splitCSV(keys),
				Endpoints: splitCSV(endpoints),
			}, cmd.OutOrStdout())
			return err
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "recordings JSON file path")
	cmd.Flags().StringVar(&algorithm, "algorithm", "", "replay algorithm: token_bucket|sliding_window|fixed_window")
	cmd.Flags().IntVar(&rate, "rate", 0, "requests allowed per window")
	cmd.Flags().StringVar(&window, "window", "", "replay window duration")
	cmd.Flags().IntVar(&burst, "burst", 0, "token bucket burst size")
	cmd.Flags().Float64Var(&speed, "speed", 0, "replay speed multiplier (0 = instant)")
	cmd.Flags().StringVar(&keys, "keys", "", "comma-separated key filter")
	cmd.Flags().StringVar(&endpoints, "endpoints", "", "comma-separated endpoint filter")
	cmd.Flags().StringVar(&configPath, "config", "", "path to Chrono JSON config file")
	_ = cmd.MarkFlagRequired("file")

	return cmd
}

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
