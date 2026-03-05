package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type KrakenResponse struct {
	Result map[string]interface{} `json:"result"`
}

func calculateWMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0.0
	}
	subset := prices[len(prices)-period:]
	var sumWeights, weightedSum float64
	for i, price := range subset {
		weight := float64(i + 1)
		weightedSum += price * weight
		sumWeights += weight
	}
	return weightedSum / sumWeights
}

func wmaCrossoverSample() {
	symbol := "XXBTZUSD" // Kraken's pair name for BTC/USD
	url := fmt.Sprintf("https://api.kraken.com/0/public/OHLC?pair=%s&interval=1440", symbol)

	resp, _ := http.Get(url)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var data KrakenResponse
	json.Unmarshal(body, &data)

	// Kraken returns data in nested arrays: [time, open, high, low, close, ...]
	ohlcData := data.Result[symbol].([]interface{})
	var closePrices []float64

	for _, bar := range ohlcData {
		b := bar.([]interface{})
		price, _ := strconv.ParseFloat(b[4].(string), 64)
		closePrices = append(closePrices, price)
	}

	fastWMA := calculateWMA(closePrices, 7)
	slowWMA := calculateWMA(closePrices, 30)
	currentPrice := closePrices[len(closePrices)-1]

	fmt.Printf("Current BTC Price: $%.2f\n", currentPrice)
	fmt.Printf("7-Day WMA: %.2f | 30-Day WMA: %.2f\n", fastWMA, slowWMA)

	if fastWMA > slowWMA {
		fmt.Println("Signal: 🚀 BULLISH (Golden Cross)")
	} else {
		fmt.Println("Signal: ⚠️ BEARISH (Death Cross)")
	}
}
