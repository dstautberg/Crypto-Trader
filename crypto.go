package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Struct for Kraken API response
type KrakenTickerResponse struct {
	Result map[string]struct {
		C []string `json:"c"`
	} `json:"result"`
}

// detectKrakenPair inspects an AssetPairs JSON blob and returns the best matching
// pair key (e.g., "XXBTZUSD") for the given krakenTicker (e.g., "XBT"). Returns
// empty string if no suitable pair is found.
func detectKrakenPair(krakenTicker string, assetPairsJSON []byte) string {
	var assetResp struct {
		Result map[string]struct {
			Altname string `json:"altname"`
		} `json:"result"`
	}
	if err := json.Unmarshal(assetPairsJSON, &assetResp); err != nil {
		return ""
	}

	targetAlt := strings.ToUpper(krakenTicker + "USD")
	for k, v := range assetResp.Result {
		if strings.ToUpper(v.Altname) == targetAlt {
			return k
		}
	}

	// Fallback: find any pair key that contains the ticker and USD
	for k := range assetResp.Result {
		ku := strings.ToUpper(k)
		if strings.Contains(ku, strings.ToUpper(krakenTicker)) && strings.Contains(ku, "USD") {
			return k
		}
	}

	return ""
}

func getBTCPrice(ticker string) (float64, string, error) {
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
	// Try to auto-detect the correct Kraken asset pair via AssetPairs API
	pairParam := ""
	assetPairsURL := "https://api.kraken.com/0/public/AssetPairs"
	apResp, apErr := http.Get(assetPairsURL)
	if apErr == nil {
		defer apResp.Body.Close()
		apBody, _ := io.ReadAll(apResp.Body)
		var assetResp struct {
			Result map[string]struct {
				Altname string `json:"altname"`
			} `json:"result"`
		}
		if jsonErr := json.Unmarshal(apBody, &assetResp); jsonErr == nil {
			// Prefer exact altname match, e.g. "XBTUSD"
			targetAlt := strings.ToUpper(krakenTicker + "USD")
			for k, v := range assetResp.Result {
				if strings.ToUpper(v.Altname) == targetAlt {
					pairParam = k
					break
				}
			}
			// Fallback: find any pair key that contains the ticker and USD
			if pairParam == "" {
				for k := range assetResp.Result {
					ku := strings.ToUpper(k)
					if strings.Contains(ku, strings.ToUpper(krakenTicker)) && strings.Contains(ku, "USD") {
						pairParam = k
						break
					}
				}
			}
		}
	}

	// Final fallback: use altname like XBTUSD which Kraken accepts
	if pairParam == "" {
		pairParam = fmt.Sprintf("%sUSD", krakenTicker)
	}

	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pairParam)
	resp, err := http.Get(url)
	if err != nil {
		return 0, url, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, url, err
	}

	var tickerResp KrakenTickerResponse
	err = json.Unmarshal(body, &tickerResp)
	if err != nil {
		return 0, url, err
	}

	for _, v := range tickerResp.Result {
		if len(v.C) > 0 {
			var price float64
			fmt.Sscanf(v.C[0], "%f", &price)
			return price, url, nil
		}
	}
	return 0, url, fmt.Errorf("price not found in response: %s", string(body))
}
