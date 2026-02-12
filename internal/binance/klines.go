package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"realtime-market-engine/internal/candle"
)

type klineRespRow []any

type KlineFetcher struct {
	baseURL string
	hc      *http.Client
}

func NewKlineFetcher() *KlineFetcher {
	return &KlineFetcher{
		baseURL: "https://api.binance.com",
		hc: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (f *KlineFetcher) FetchKlines(ctx context.Context, symbol, interval string, start, end time.Time) ([]candle.Candle, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}
	if interval == "" {
		return nil, fmt.Errorf("interval required")
	}

	var out []candle.Candle

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	if endMs <= startMs {
		return nil, fmt.Errorf("end must be after start")
	}

	limit := int64(1000)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		u, _ := url.Parse(f.baseURL)
		u.Path = "/api/v3/klines"
		q := u.Query()
		q.Set("symbol", symbol)
		q.Set("interval", interval)
		q.Set("limit", strconv.FormatInt(limit, 10))
		q.Set("startTime", strconv.FormatInt(startMs, 10))
		q.Set("endTime", strconv.FormatInt(endMs, 10))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}

		resp, err := f.hc.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("binance klines http %d: %s", resp.StatusCode, string(b))
		}

		var rows []klineRespRow
		if err := json.Unmarshal(b, &rows); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}

		var lastCloseMs int64
		for _, r := range rows {
			if len(r) < 7 {
				continue
			}
			openMs, ok := toInt64(r[0])
			if !ok {
				continue
			}
			open, ok := toFloat(r[1])
			if !ok {
				continue
			}
			high, ok := toFloat(r[2])
			if !ok {
				continue
			}
			low, ok := toFloat(r[3])
			if !ok {
				continue
			}
			closeP, ok := toFloat(r[4])
			if !ok {
				continue
			}
			closeMs, ok := toInt64(r[6])
			if !ok {
				continue
			}
			startT := time.UnixMilli(openMs)
			endT := time.UnixMilli(closeMs)

			out = append(out, candle.Candle{
				Symbol:    symbol,
				Start:     startT,
				End:       endT,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     closeP,
				Timestamp: endT,
			})
			lastCloseMs = closeMs
		}

		if lastCloseMs == 0 {
			break
		}

		nextStart := lastCloseMs + 1
		if nextStart >= endMs {
			break
		}
		startMs = nextStart

		if len(rows) < int(limit) {
			break
		}

		time.Sleep(150 * time.Millisecond)
	}

	return out, nil
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
