package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

func computeWMA(prices []float64, window int) []float64 {
	n := len(prices)
	if n == 0 {
		return nil
	}
	if window < 1 {
		window = 1
	}
	res := make([]float64, n)
	for i := 0; i < n; i++ {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		var weightedSum, sumWeights float64
		for j := start; j <= i; j++ {
			weight := float64(j - start + 1)
			weightedSum += prices[j] * weight
			sumWeights += weight
		}
		if sumWeights == 0 {
			res[i] = prices[i]
		} else {
			res[i] = weightedSum / sumWeights
		}
	}
	return res
}

func main() {
	days := flag.Int("days", 30, "Number of days to consider when estimating samples per day for WMA windows")
	dry := flag.Bool("dry", false, "Dry run: don't insert into DB")
	flag.Parse()

	db, err := sql.Open("sqlite", "btc_prices.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := fmt.Sprintf("SELECT id, price, timestamp FROM btc_price WHERE timestamp >= datetime('now', '-%d day') ORDER BY timestamp ASC", *days)
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	type P struct {
		ID        int
		Price     float64
		Timestamp string
	}
	var prices []P
	for rows.Next() {
		var p P
		if err := rows.Scan(&p.ID, &p.Price, &p.Timestamp); err != nil {
			continue
		}
		prices = append(prices, p)
	}
	if len(prices) == 0 {
		log.Fatal("no prices found")
	}

	// Build raw price slice
	raw := make([]float64, len(prices))
	for i := range prices {
		raw[i] = prices[i].Price
	}

	// Estimate samples per day
	samplesPerDay := float64(len(raw)) / float64(*days)
	if samplesPerDay < 1.0 {
		samplesPerDay = 1.0
	}

	window7 := int(samplesPerDay * 7.0)
	window30 := int(samplesPerDay * 30.0)
	if window7 < 1 {
		window7 = 1
	}
	if window30 < 1 {
		window30 = 1
	}
	if window7 > len(raw) {
		window7 = len(raw)
	}
	if window30 > len(raw) {
		window30 = len(raw)
	}

	// If windows are too large relative to available samples, cap them to smaller fractions
	if window7 > len(raw)/2 {
		window7 = len(raw) / 10
		if window7 < 1 {
			window7 = 1
		}
	}
	if window30 > len(raw)/2 {
		window30 = len(raw) / 5
		if window30 < 1 {
			window30 = 1
		}
	}

	fmt.Printf("Processing %d price points, samples/day=%.2f, window7=%d, window30=%d\n", len(raw), samplesPerDay, window7, window30)

	wma7 := computeWMA(raw, window7)
	wma30 := computeWMA(raw, window30)

	inserted := 0
	for i := 1; i < len(raw); i++ {
		prev7 := wma7[i-1]
		prev30 := wma30[i-1]
		cur7 := wma7[i]
		cur30 := wma30[i]
		action := ""
		if prev7 <= prev30 && cur7 > cur30 {
			action = "BUY"
		} else if prev7 >= prev30 && cur7 < cur30 {
			action = "SELL"
		}
		if action == "" {
			continue
		}

		// Check if a signal already exists for this price_id and action
		var cnt int
		err := db.QueryRow("SELECT COUNT(1) FROM trading_signals WHERE price_id = ? AND action = ?", prices[i].ID, action).Scan(&cnt)
		if err != nil {
			log.Fatal(err)
		}
		if cnt > 0 {
			// already exists
			continue
		}

		if *dry {
			fmt.Printf("DRY: would insert %s at price_id=%d price=%.2f ts=%s\n", action, prices[i].ID, prices[i].Price, prices[i].Timestamp)
			inserted++
			continue
		}

		_, err = db.Exec("INSERT INTO trading_signals (price_id, action, price, timestamp) VALUES (?, ?, ?, ?)", prices[i].ID, action, prices[i].Price, time.Now().UTC())
		if err != nil {
			log.Fatal(err)
		}
		inserted++
	}

	fmt.Printf("Inserted %d new signals\n", inserted)

	// print environment variable hint for cleanup
	if !*dry {
		fmt.Println("Tip: to remove inserted signals for testing, run tools/delete_test_signals.go with appropriate price_id list.")
	}
	// exit with code 0
	os.Exit(0)
}
