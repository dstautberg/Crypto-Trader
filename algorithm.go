package main

import (
	"database/sql"
	"fmt"
)

// TradingSignal represents the recommendation from the algorithm
type TradingSignal struct {
	Action         string  // "BUY", "SELL", or "HOLD"
	CurrentPrice   float64
	MovingAverage  float64
	PercentChange  float64
	Recommendation string
}

// TradingAlgorithm analyzes price data and generates trading signals based on WMA crossover
func TradingAlgorithm(db *sql.DB, currentPrice float64, movingAvgDays int, changeThreshold float64) (*TradingSignal, error) {
	// Fetch price data for WMA calculation
	query := `SELECT price FROM btc_price ORDER BY timestamp DESC LIMIT 240`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer rows.Close()

	var prices []float64
	for rows.Next() {
		var p float64
		if err := rows.Scan(&p); err != nil {
			continue
		}
		prices = append([]float64{p}, prices...) // prepend to maintain chronological order
	}

	// Need at least 30 data points for reliable signals
	if len(prices) < 30 {
		return &TradingSignal{
			Action:         "HOLD",
			CurrentPrice:   currentPrice,
			MovingAverage:  0,
			PercentChange:  0,
			Recommendation: "",
		}, nil
	}

	// Estimate samples per day
	samplesPerDay := float64(len(prices)) / 1.0 // Assumes fetched data spans approximately 1 day
	if samplesPerDay < 1 {
		samplesPerDay = 1
	}

	// Window sizes for 7-day and 30-day WMAs
	window7 := int(samplesPerDay * 7.0)
	window30 := int(samplesPerDay * 30.0)

	if window7 < 1 {
		window7 = 1
	}
	if window30 < 1 {
		window30 = 1
	}
	if window7 > len(prices) {
		window7 = len(prices)
	}
	if window30 > len(prices) {
		window30 = len(prices)
	}

	// Calculate WMAs
	wma7 := computeWMA(prices, window7)
	wma30 := computeWMA(prices, window30)

	// Get current and previous WMA values
	currentWMA7 := wma7[len(wma7)-1]
	currentWMA30 := wma30[len(wma30)-1]
	prevWMA7 := wma7[len(wma7)-2]
	prevWMA30 := wma30[len(wma30)-2]

	// Calculate simple percent change for reference
	avgPrice := (currentWMA7 + currentWMA30) / 2.0
	percentChange := ((currentPrice - avgPrice) / avgPrice) * 100

	signal := &TradingSignal{
		CurrentPrice:  currentPrice,
		MovingAverage: avgPrice,
		PercentChange: percentChange,
	}

	// Detect crossovers
	// Golden Cross: 7d WMA crosses above 30d WMA (BUY signal)
	if prevWMA7 <= prevWMA30 && currentWMA7 > currentWMA30 {
		signal.Action = "BUY"
		signal.Recommendation = "** BUY **"
	} else if prevWMA7 >= prevWMA30 && currentWMA7 < currentWMA30 {
		// Death Cross: 7d WMA crosses below 30d WMA (SELL signal)
		signal.Action = "SELL"
		signal.Recommendation = "** SELL **"
	} else {
		signal.Action = "HOLD"
		signal.Recommendation = ""
	}

	return signal, nil
}
