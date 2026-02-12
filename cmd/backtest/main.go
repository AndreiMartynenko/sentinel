package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"realtime-market-engine/internal/backtest"
	"realtime-market-engine/internal/binance"
)

func main() {
	var symbol string
	var interval string
	var start string
	var end string

	var initialEquity float64
	var fee float64
	var slippage float64
	var allowShort bool

	var emaFast int
	var emaSlow int
	var trendConfirm int
	var trendMinDiff float64
	var trendCooldown time.Duration

	var breakoutLookback time.Duration
	var breakoutPct float64
	var breakoutCooldown time.Duration

	var stopLoss float64
	var takeProfit float64

	flag.StringVar(&symbol, "symbol", "BTCUSDT", "Binance symbol")
	flag.StringVar(&interval, "interval", "1m", "Binance kline interval (e.g. 1m,5m)")
	flag.StringVar(&start, "start", "", "Start time RFC3339 (e.g. 2026-01-01T00:00:00Z)")
	flag.StringVar(&end, "end", "", "End time RFC3339 (e.g. 2026-01-02T00:00:00Z)")

	flag.Float64Var(&initialEquity, "equity", 1000, "Initial equity in quote currency")
	flag.Float64Var(&fee, "fee", 0.001, "Fee rate per side (0.001 = 0.1%)")
	flag.Float64Var(&slippage, "slippage", 0.0002, "Slippage rate per fill (0.0002 = 2 bps)")
	flag.BoolVar(&allowShort, "short", false, "Allow short trades")

	flag.IntVar(&emaFast, "ema-fast", 20, "Fast EMA window (candles)")
	flag.IntVar(&emaSlow, "ema-slow", 50, "Slow EMA window (candles)")
	flag.IntVar(&trendConfirm, "trend-confirm", 3, "Confirm trend flip after N consecutive candles")
	flag.Float64Var(&trendMinDiff, "trend-min-diff", 0.0, "Minimum relative EMA separation (abs(fast-slow)/price) to confirm flip")
	flag.DurationVar(&trendCooldown, "trend-cooldown", 0, "Minimum time between trend flip notifications")

	flag.DurationVar(&breakoutLookback, "breakout-lookback", 5*time.Minute, "Breakout lookback window")
	flag.Float64Var(&breakoutPct, "breakout-pct", 0.001, "Breakout threshold fraction")
	flag.DurationVar(&breakoutCooldown, "breakout-cooldown", 0, "Minimum time between breakout signals")

	flag.Float64Var(&stopLoss, "sl", 0.003, "Stop loss percent (0.003 = 0.3%)")
	flag.Float64Var(&takeProfit, "tp", 0.006, "Take profit percent (0.006 = 0.6%)")

	flag.Parse()

	if start == "" || end == "" {
		log.Fatalf("-start and -end are required")
	}

	st, err := time.Parse(time.RFC3339, start)
	if err != nil {
		log.Fatalf("invalid -start: %v", err)
	}
	et, err := time.Parse(time.RFC3339, end)
	if err != nil {
		log.Fatalf("invalid -end: %v", err)
	}

	ctx := context.Background()
	fetcher := binance.NewKlineFetcher()
	candles, err := fetcher.FetchKlines(ctx, symbol, interval, st, et)
	if err != nil {
		log.Fatalf("fetch klines: %v", err)
	}
	if len(candles) == 0 {
		log.Fatalf("no candles fetched")
	}

	res, err := backtest.Run(candles, backtest.Config{
		InitialEquity: initialEquity,
		FeeRate:       fee,
		SlippageRate:  slippage,
		AllowShort:    allowShort,
		StopLossPct:   stopLoss,
		TakeProfitPct: takeProfit,
		EmaFast:       emaFast,
		EmaSlow:       emaSlow,
		TrendConfirm:  trendConfirm,
		TrendMinDiff:  trendMinDiff,
		TrendCooldown: trendCooldown,
		BreakoutLookback: breakoutLookback,
		BreakoutPct:      breakoutPct,
		BreakoutCooldown: breakoutCooldown,
	})
	if err != nil {
		log.Fatalf("backtest: %v", err)
	}

	fmt.Printf("Candles: %d\n", len(candles))
	fmt.Printf("Trades: %d\n", len(res.Trades))
	fmt.Printf("Final equity: %.2f\n", res.FinalEquity)
	fmt.Printf("Total return: %.2f%%\n", res.TotalReturn*100)
	fmt.Printf("Max drawdown: %.2f%%\n", res.MaxDrawdown*100)
	fmt.Printf("Win rate: %.2f%%\n", res.WinRate*100)
	fmt.Printf("Profit factor: %.3f\n", res.ProfitFactor)
}
