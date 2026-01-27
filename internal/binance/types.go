package binance

// I need this from Binance
type KlineMessage struct {
	Stream string `json:"stream"` // btcusdt@kline_1m
	Data   struct {
		Kline struct {
			Symbol    string `json:"symbol"`    //"BTCUSDT"
			Close     string `json:"close"`     // "87000.00"
			CloseTime int64  `json:"closeTime"` // in milliseconds
			IsClosed  bool   `json:"isClosed"`  // true/false
		} `json:"kline"`
	} `json:"data"`
}
