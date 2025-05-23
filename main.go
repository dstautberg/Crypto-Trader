package main

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"fmt"
)

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

	fmt.Println("SQLite database (without CGO) is ready.")
}
