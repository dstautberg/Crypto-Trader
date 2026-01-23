package main

import (
	"database/sql"
	"fmt"
)

// TradingSignal represents the recommendation from the algorithm
type TradingSignal struct {
	Action         string // "BUY", "SELL", or "HOLD"
	CurrentPrice   float64
	MovingAverage  float64
	PercentChange  float64
	Recommendation string
}

// TradingAlgorithm analyzes price data and generates trading signals
func TradingAlgorithm(db *sql.DB, currentPrice float64, movingAvgDays int, changeThreshold float64) (*TradingSignal, error) {
	// Calculate moving average for the last N days
	query := fmt.Sprintf(`SELECT AVG(price) FROM btc_price WHERE timestamp >= datetime('now', '-%d day')`, movingAvgDays)
	row := db.QueryRow(query)
	var avgNDays sql.NullFloat64
	if err := row.Scan(&avgNDays); err != nil {
		return nil, fmt.Errorf("failed to calculate moving average: %w", err)
	}

	// If no moving average data available, return HOLD
	if !avgNDays.Valid || avgNDays.Float64 == 0 {
		return &TradingSignal{
			Action:         "HOLD",
			CurrentPrice:   currentPrice,
			MovingAverage:  0,
			PercentChange:  0,
			Recommendation: "",
		}, nil
	}

	// Calculate percent change
	percentChange := ((currentPrice - avgNDays.Float64) / avgNDays.Float64) * 100

	// Determine trading action
	signal := &TradingSignal{
		CurrentPrice:  currentPrice,
		MovingAverage: avgNDays.Float64,
		PercentChange: percentChange,
	}

	if percentChange > changeThreshold {
		signal.Action = "SELL"
		signal.Recommendation = "** SELL **"
	} else if percentChange < -changeThreshold {
		signal.Action = "BUY"
		signal.Recommendation = "** BUY **"
	} else {
		signal.Action = "HOLD"
		signal.Recommendation = ""
	}

	return signal, nil
}
