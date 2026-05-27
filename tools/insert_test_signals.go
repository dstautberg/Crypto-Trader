package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "btc_prices.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get last two price IDs
	rows, err := db.Query("SELECT id, price FROM btc_price ORDER BY id DESC LIMIT 2")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var ids []int
	var prices []float64
	for rows.Next() {
		var id int
		var price float64
		if err := rows.Scan(&id, &price); err != nil {
			continue
		}
		ids = append(ids, id)
		prices = append(prices, price)
	}
	if len(ids) == 0 {
		log.Fatal("no price rows found")
	}

	// Insert a BUY for the most recent price, and a SELL for the second-most recent (if available)
	now := time.Now().UTC()
	res, err := db.Exec("INSERT INTO trading_signals (price_id, action, price, timestamp) VALUES (?, ?, ?, ?)", ids[0], "BUY", prices[0], now)
	if err != nil {
		log.Fatal(err)
	}
	lastID, _ := res.LastInsertId()
	fmt.Printf("Inserted BUY signal id=%d for price_id=%d\n", lastID, ids[0])
	if len(ids) > 1 {
		res2, err := db.Exec("INSERT INTO trading_signals (price_id, action, price, timestamp) VALUES (?, ?, ?, ?)", ids[1], "SELL", prices[1], now)
		if err != nil {
			log.Fatal(err)
		}
		lastID2, _ := res2.LastInsertId()
		fmt.Printf("Inserted SELL signal id=%d for price_id=%d\n", lastID2, ids[1])
	}

	fmt.Println("Done")
}
