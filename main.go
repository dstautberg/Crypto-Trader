package main

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Struct for Kraken API response
type KrakenTickerResponse struct {
	Result map[string]struct {
		C []string `json:"c"`
	} `json:"result"`
}

func getBTCPrice() (float64, error) {
	resp, err := http.Get("https://api.kraken.com/0/public/Ticker?pair=XXBTZUSD")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var ticker KrakenTickerResponse
	err = json.Unmarshal(body, &ticker)
	if err != nil {
		return 0, err
	}

	for _, v := range ticker.Result {
		if len(v.C) > 0 {
			var price float64
			fmt.Sscanf(v.C[0], "%f", &price)
			return price, nil
		}
	}
	return 0, fmt.Errorf("price not found in response")
}

func main() {
	db, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS example (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS btc_price (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		price REAL,
		timestamp DATETIME
	)`)
	if err != nil {
		panic(err)
	}

	price, err := getBTCPrice()
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(`INSERT INTO btc_price (price, timestamp) VALUES (?, ?)`, price, time.Now().UTC())
	if err != nil {
		panic(err)
	}

	fmt.Printf("Saved BTC price: $%.2f\n", price)

	// Display all saved BTC prices
	rows, err := db.Query(`SELECT id, price, timestamp FROM btc_price ORDER BY timestamp DESC`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fmt.Println("\nSaved BTC Prices:")
	for rows.Next() {
		var id int
		var price float64
		var timestamp string
		if err := rows.Scan(&id, &price, &timestamp); err != nil {
			panic(err)
		}
		fmt.Printf("ID: %d | Price: $%.2f | Timestamp: %s\n", id, price, timestamp)
	}

	// Calculate and display the moving average for the last 24 hours
	row := db.QueryRow(`SELECT AVG(price) FROM btc_price WHERE timestamp >= datetime('now', '-1 day')`)
	var avg24h sql.NullFloat64
	if err := row.Scan(&avg24h); err != nil {
		panic(err)
	}
	if avg24h.Valid {
		fmt.Printf("\n24h Moving Average BTC Price: $%.2f\n", avg24h.Float64)
	} else {
		fmt.Println("\nNo BTC price data for the last 24 hours.")
	}
}
