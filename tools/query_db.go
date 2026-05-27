package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "btc_prices.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("Recent btc_price rows:")
	rows, err := db.Query("SELECT id, price, timestamp FROM btc_price ORDER BY id DESC LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var price float64
		var ts string
		if err := rows.Scan(&id, &price, &ts); err != nil {
			continue
		}
		fmt.Printf("id=%d price=%.2f ts=%s\n", id, price, ts)
	}

	fmt.Println("\nRecent trading_signals:")
	sRows, err := db.Query("SELECT id, price_id, action, price, timestamp FROM trading_signals ORDER BY id DESC LIMIT 20")
	if err != nil {
		log.Fatal(err)
	}
	defer sRows.Close()
	for sRows.Next() {
		var id, priceID int
		var action string
		var price float64
		var ts string
		if err := sRows.Scan(&id, &priceID, &action, &price, &ts); err != nil {
			continue
		}
		fmt.Printf("id=%d price_id=%d action=%s price=%.2f ts=%s\n", id, priceID, action, price, ts)
	}
}
