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

	// Delete signals we inserted for testing (price_id 142889, 142890)
	res, err := db.Exec("DELETE FROM trading_signals WHERE price_id IN (?, ?) AND action IN ('BUY','SELL')", 142890, 142889)
	if err != nil {
		log.Fatal(err)
	}
	count, _ := res.RowsAffected()
	fmt.Printf("Deleted %d test signal(s)\n", count)
}
