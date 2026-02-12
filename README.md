# realtime-market-engine

A small Go service that:

- Streams **BTCUSDT** market data from Binance (WebSocket)
- Keeps the latest price in an in-memory store
- Exposes the latest price via HTTP
- Broadcasts live events via WebSocket (`/ws`)
- Detects:
  - **EMA trend flips** (up/down)
  - **Breakouts** using aggregated micro-candles
- Includes a minimal **backtester** that replays historical Binance klines

## Requirements

- Go **1.22+**

## Run the live engine

```bash
go run ./cmd/engine -http :8080
```

Open:

- `http://localhost:8080/` (simple live view that connects to `/ws`)
- `http://localhost:8080/health`
- `http://localhost:8080/prices/BTCUSDT`
- WebSocket: `ws://localhost:8080/ws`

### Engine flags

#### HTTP

- `-http` (default `:8080`)

#### Trend detection (EMA crossover)

- `-ema-fast` (default `20`) fast EMA window in ticks
- `-ema-slow` (default `50`) slow EMA window in ticks
- `-trend-confirm` (default `3`) consecutive ticks required to confirm flip
- `-trend-min-diff` (default `0.00005`) minimum separation required to confirm flip: `abs(fast-slow)/price`
- `-trend-cooldown` (default `10s`) minimum time between trend notifications

#### Breakout detection (micro-candles)

- `-candle-interval` (default `5s`) candle aggregation interval
- `-breakout-lookback` (default `5m`) lookback window for high/low breakout levels
- `-breakout-pct` (default `0.001`) breakout threshold (0.001 = 0.1%)
- `-breakout-cooldown` (default `30s`) minimum time between breakout notifications

Example:

```bash
go run ./cmd/engine \
  -http :8080 \
  -ema-fast 20 -ema-slow 50 \
  -trend-confirm 3 -trend-min-diff 0.0001 -trend-cooldown 20s \
  -candle-interval 5s \
  -breakout-lookback 5m -breakout-pct 0.001 -breakout-cooldown 30s
```

## WebSocket events (`/ws`)

The `/ws` stream publishes JSON messages for multiple event types.

### Price ticks

Emitted on every incoming Binance tick.

```json
{"Symbol":"BTCUSDT","Price":96500.12,"Timestamp":"2026-02-08T10:00:00Z","Source":"binance"}
```

### Trend flips

Emitted when a trend flip is confirmed.

```json
{"type":"trend_change","symbol":"BTCUSDT","trend":"down","fastEma":96490.1,"slowEma":96510.7,"price":96480.3,"timestamp":"2026-02-08T10:00:05Z"}
```

### Breakouts

Emitted when a completed candle closes beyond the previous lookback high/low by `breakoutPct`.

```json
{"type":"breakout","symbol":"BTCUSDT","dir":"up","price":96550.1,"level":96480.0,"pct":0.001,"lookback":"5m0s","candleEnd":"2026-02-08T10:00:10Z","timestamp":"2026-02-08T10:00:10Z"}
```

## Run the backtester

The backtester downloads historical Binance klines (no API key required) and runs a minimal strategy simulation.

```bash
go run ./cmd/backtest \
  -symbol BTCUSDT \
  -interval 1m \
  -start 2026-02-01T00:00:00Z \
  -end   2026-02-02T00:00:00Z
```

### Backtest flags

- `-symbol` (default `BTCUSDT`)
- `-interval` (default `1m`) Binance kline interval
- `-start` / `-end` (RFC3339, required)

Costs:

- `-fee` (default `0.001`) fee rate per side (0.001 = 0.1%)
- `-slippage` (default `0.0002`) slippage per fill (0.0002 = 2 bps)

Strategy parameters:

- `-ema-fast`, `-ema-slow`
- `-breakout-lookback`, `-breakout-pct`, `-breakout-cooldown`
- `-sl` stop loss percent (default `0.003` = 0.3%)
- `-tp` take profit percent (default `0.006` = 0.6%)
- `-short` enable short trades

Example:

```bash
go run ./cmd/backtest \
  -symbol BTCUSDT \
  -interval 5m \
  -start 2026-02-01T00:00:00Z \
  -end   2026-02-08T00:00:00Z \
  -ema-fast 20 -ema-slow 50 \
  -breakout-lookback 2h \
  -breakout-pct 0.003 \
  -sl 0.006 -tp 0.012 \
  -fee 0.001 -slippage 0.0002
```

## Notes

- This project is a research/prototype tool. No profitability is guaranteed.
- Live engine currently listens to a single symbol: `BTCUSDT`.
