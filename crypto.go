package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Struct for Kraken API response
type KrakenTickerResponse struct {
	Result map[string]struct {
		C []string `json:"c"`
	} `json:"result"`
}

func getBTCPrice(ticker string) (float64, error) {
	// Kraken expects XBT for BTC, ETH for Ethereum, etc. Map common names to Kraken codes.
	tickerMap := map[string]string{
		"BTC": "XBT",
		"ETH": "ETH",
		"LTC": "LTC",
		// Add more mappings as needed
	}
	krakenTicker := ticker
	if val, ok := tickerMap[ticker]; ok {
		krakenTicker = val
	}
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=X%sZUSD", krakenTicker)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var tickerResp KrakenTickerResponse
	err = json.Unmarshal(body, &tickerResp)
	if err != nil {
		return 0, err
	}

	for _, v := range tickerResp.Result {
		if len(v.C) > 0 {
			var price float64
			fmt.Sscanf(v.C[0], "%f", &price)
			return price, nil
		}
	}
	return 0, fmt.Errorf("price not found in response: %s", string(body))
}
