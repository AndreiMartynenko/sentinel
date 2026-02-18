package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"realtime-market-engine/internal/alert"
	"realtime-market-engine/internal/binance"
	"realtime-market-engine/internal/candle"
	"realtime-market-engine/internal/httpapi"
	"realtime-market-engine/internal/store"
	"realtime-market-engine/internal/trend"
)

func main() {
	var httpAddr string
	var emaFast int
	var emaSlow int
	var confirmTicks int
	var trendMinDiff float64
	var trendCooldown time.Duration
	var candleInterval time.Duration
	var breakoutLookback time.Duration
	var breakoutPct float64
	var breakoutCooldown time.Duration
	flag.StringVar(&httpAddr, "http", ":8080", "HTTP listen address")
	flag.IntVar(&emaFast, "ema-fast", 20, "Fast EMA window (ticks)")
	flag.IntVar(&emaSlow, "ema-slow", 50, "Slow EMA window (ticks)")
	flag.IntVar(&confirmTicks, "trend-confirm", 3, "Confirm trend flip after N consecutive ticks")
	flag.Float64Var(&trendMinDiff, "trend-min-diff", 0.00005, "Minimum relative EMA separation (abs(fast-slow)/price) required to confirm a trend flip")
	flag.DurationVar(&trendCooldown, "trend-cooldown", 10*time.Second, "Minimum time between trend flip notifications")
	flag.DurationVar(&candleInterval, "candle-interval", 5*time.Second, "Candle aggregation interval")
	flag.DurationVar(&breakoutLookback, "breakout-lookback", 5*time.Minute, "Breakout lookback window (uses completed candles)")
	flag.Float64Var(&breakoutPct, "breakout-pct", 0.001, "Breakout threshold as a fraction (0.001 = 0.1%)")
	flag.DurationVar(&breakoutCooldown, "breakout-cooldown", 30*time.Second, "Minimum time between breakout notifications")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.NewPriceStore()
	hub := httpapi.NewHub()
	go hub.Run(ctx)

	detector := trend.NewEMACrossoverDetector(emaFast, emaSlow, confirmTicks, trendMinDiff, trendCooldown)
	agg := candle.NewAggregator(candleInterval)
	breakout := alert.NewBreakoutDetector(breakoutLookback, breakoutPct, breakoutCooldown)

	events, err := binance.StartAggTradeListener(ctx, "BTCUSDT")
	if err != nil {
		log.Fatalf("binance listener error: %v", err)
	}

	go func() {
		for ev := range events {
			st.Update(ev)
			hub.PublishPrice(ev)

			if c, ok := agg.Push(ev); ok {
				if bo, ok := breakout.Push(c); ok {
					b, err := json.Marshal(bo)
					if err == nil {
						hub.PublishJSON(b)
					}
					log.Printf("breakout: %s %s", bo.Symbol, bo.Dir)
				}
			}

			if change, ok := detector.Push(ev); ok {
				b, err := json.Marshal(change)
				if err == nil {
					hub.PublishJSON(b)
				}
				log.Printf("trend change: %s", change.Trend)
			}
		}
	}()

	mux := http.NewServeMux()
	routes := httpapi.NewRoutes(st, hub)
	routes.Register(mux)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("engine listening on %s", httpAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}
