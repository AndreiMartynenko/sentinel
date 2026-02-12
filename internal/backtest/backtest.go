package backtest

import (
	"fmt"
	"math"
	"time"

	"realtime-market-engine/internal/alert"
	"realtime-market-engine/internal/candle"
	"realtime-market-engine/internal/trend"
	"realtime-market-engine/internal/types"
)

type Side string

const (
	SideLong  Side = "long"
	SideShort Side = "short"
)

type Config struct {
	InitialEquity float64
	FeeRate       float64
	SlippageRate  float64

	AllowShort bool

	StopLossPct   float64
	TakeProfitPct float64

	EmaFast int
	EmaSlow int

	TrendConfirm  int
	TrendMinDiff  float64
	TrendCooldown time.Duration

	BreakoutLookback time.Duration
	BreakoutPct      float64
	BreakoutCooldown time.Duration
}

type Trade struct {
	Side     Side
	EntryT   time.Time
	Entry    float64
	ExitT    time.Time
	Exit     float64
	Reason   string
	GrossPnL float64
	NetPnL   float64
}

type Result struct {
	Trades       []Trade
	FinalEquity  float64
	TotalReturn  float64
	MaxDrawdown  float64
	WinRate      float64
	ProfitFactor float64
}

type position struct {
	side      Side
	entry     float64
	entryTime time.Time
	qty       float64
	stop      float64
	tp        float64
}

func Run(candles []candle.Candle, cfg Config) (Result, error) {
	if len(candles) < 10 {
		return Result{}, fmt.Errorf("not enough candles")
	}
	if cfg.InitialEquity <= 0 {
		cfg.InitialEquity = 1000
	}
	if cfg.StopLossPct <= 0 {
		cfg.StopLossPct = 0.003
	}
	if cfg.TakeProfitPct <= 0 {
		cfg.TakeProfitPct = 0.006
	}
	if cfg.EmaFast <= 0 {
		cfg.EmaFast = 20
	}
	if cfg.EmaSlow <= 0 {
		cfg.EmaSlow = 50
	}
	if cfg.TrendConfirm <= 0 {
		cfg.TrendConfirm = 3
	}
	if cfg.BreakoutLookback <= 0 {
		cfg.BreakoutLookback = 5 * time.Minute
	}
	if cfg.BreakoutCooldown < 0 {
		cfg.BreakoutCooldown = 0
	}
	if cfg.TrendCooldown < 0 {
		cfg.TrendCooldown = 0
	}

	equity := cfg.InitialEquity
	peakEquity := equity
	maxDD := 0.0

	det := trend.NewEMACrossoverDetector(cfg.EmaFast, cfg.EmaSlow, cfg.TrendConfirm, cfg.TrendMinDiff, cfg.TrendCooldown)
	bo := alert.NewBreakoutDetector(cfg.BreakoutLookback, cfg.BreakoutPct, cfg.BreakoutCooldown)

	trendDir := trend.DirectionUp

	var pos *position
	var trades []Trade

	fee := func(notional float64) float64 {
		if cfg.FeeRate <= 0 {
			return 0
		}
		return math.Abs(notional) * cfg.FeeRate
	}

	applySlip := func(price float64, side Side, isEntry bool) float64 {
		if cfg.SlippageRate <= 0 {
			return price
		}
		m := 1.0
		slip := cfg.SlippageRate
		if side == SideLong {
			if isEntry {
				m = 1.0 + slip
			} else {
				m = 1.0 - slip
			}
		} else {
			if isEntry {
				m = 1.0 - slip
			} else {
				m = 1.0 + slip
			}
		}
		return price * m
	}

	exitPos := func(t time.Time, exitPx float64, reason string) {
		if pos == nil {
			return
		}

		exitPx = applySlip(exitPx, pos.side, false)
		notionalIn := pos.entry * pos.qty
		notionalOut := exitPx * pos.qty

		gross := 0.0
		if pos.side == SideLong {
			gross = notionalOut - notionalIn
		} else {
			gross = notionalIn - notionalOut
		}

		net := gross - fee(notionalIn) - fee(notionalOut)
		equity += net

		trades = append(trades, Trade{
			Side:     pos.side,
			EntryT:   pos.entryTime,
			Entry:    pos.entry,
			ExitT:    t,
			Exit:     exitPx,
			Reason:   reason,
			GrossPnL: gross,
			NetPnL:   net,
		})
		pos = nil
	}

	for _, c := range candles {
		if pos != nil {
			if pos.side == SideLong {
				if c.Low <= pos.stop {
					exitPos(c.End, pos.stop, "stop")
				} else if c.High >= pos.tp {
					exitPos(c.End, pos.tp, "take_profit")
				}
			} else {
				if c.High >= pos.stop {
					exitPos(c.End, pos.stop, "stop")
				} else if c.Low <= pos.tp {
					exitPos(c.End, pos.tp, "take_profit")
				}
			}
		}

		tick := typesPriceEventFromCandle(c)
		_, _ = det.Push(tick)
		if dir, ok := det.CurrentDirection(); ok {
			trendDir = dir
		}

		boEv, ok := bo.Push(c)
		if ok && pos == nil {
			if boEv.Dir == alert.BreakoutUp && trendDir == trend.DirectionUp {
				entry := applySlip(c.Close, SideLong, true)
				qty := equity / entry
				pos = &position{
					side:      SideLong,
					entry:     entry,
					entryTime: c.End,
					qty:       qty,
					stop:      entry * (1 - cfg.StopLossPct),
					tp:        entry * (1 + cfg.TakeProfitPct),
				}
			} else if cfg.AllowShort && boEv.Dir == alert.BreakoutDown && trendDir == trend.DirectionDown {
				entry := applySlip(c.Close, SideShort, true)
				qty := equity / entry
				pos = &position{
					side:      SideShort,
					entry:     entry,
					entryTime: c.End,
					qty:       qty,
					stop:      entry * (1 + cfg.StopLossPct),
					tp:        entry * (1 - cfg.TakeProfitPct),
				}
			}
		}

		if equity > peakEquity {
			peakEquity = equity
		}
		dd := (peakEquity - equity) / peakEquity
		if dd > maxDD {
			maxDD = dd
		}
	}

	if pos != nil {
		exitPos(candles[len(candles)-1].End, candles[len(candles)-1].Close, "eod")
	}

	wins := 0
	grossWin := 0.0
	grossLoss := 0.0
	for _, tr := range trades {
		if tr.NetPnL > 0 {
			wins++
			grossWin += tr.NetPnL
		} else if tr.NetPnL < 0 {
			grossLoss += -tr.NetPnL
		}
	}

	wr := 0.0
	if len(trades) > 0 {
		wr = float64(wins) / float64(len(trades))
	}

	pf := 0.0
	if grossLoss > 0 {
		pf = grossWin / grossLoss
	}

	return Result{
		Trades:       trades,
		FinalEquity:  equity,
		TotalReturn:  (equity - cfg.InitialEquity) / cfg.InitialEquity,
		MaxDrawdown:  maxDD,
		WinRate:      wr,
		ProfitFactor: pf,
	}, nil
}

func typesPriceEventFromCandle(c candle.Candle) types.PriceEvent {
	return types.PriceEvent{
		Symbol:    c.Symbol,
		Price:     c.Close,
		Timestamp: c.End,
		Source:    "backtest",
	}
}
