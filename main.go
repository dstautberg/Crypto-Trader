package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"golang.org/x/text/message"

	_ "modernc.org/sqlite"
)

// Struct for Kraken API response
type KrakenTickerResponse struct {
	Result map[string]struct {
		C []string `json:"c"`
	} `json:"result"`
}

func getBTCPrice(ticker string) (float64, error) {
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
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=X%sZUSD", krakenTicker)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var tickerResp KrakenTickerResponse
	err = json.Unmarshal(body, &tickerResp)
	if err != nil {
		return 0, err
	}

	for _, v := range tickerResp.Result {
		if len(v.C) > 0 {
			var price float64
			fmt.Sscanf(v.C[0], "%f", &price)
			return price, nil
		}
	}
	return 0, fmt.Errorf("price not found in response: %s", string(body))
}

// Get historical prices from Kraken
func getKrakenHistoricalPrices(pair string, interval int) ([]float64, error) {
	url := fmt.Sprintf("https://api.kraken.com/0/public/OHLC?pair=%s&interval=%d", pair, interval)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var prices []float64
	for k, v := range result.Result {
		if k == "last" {
			continue
		}
		if ohlcArr, ok := v.([]interface{}); ok {
			for _, entry := range ohlcArr {
				if entryArr, ok := entry.([]interface{}); ok && len(entryArr) > 4 {
					if closeStr, ok := entryArr[4].(string); ok {
						if close, err := strconv.ParseFloat(closeStr, 64); err == nil {
							prices = append(prices, close)
						}
					}
				}
			}
		}
	}
	return prices, nil
}

// Get historical price for BTC at a specific date and time (UTC)
func getBTCPriceAtDatetime(pair string, interval int, target time.Time) (float64, error) {
	// Fetch OHLC data from Kraken for the given pair and interval
	url := fmt.Sprintf("https://api.kraken.com/0/public/OHLC?pair=%s&interval=%d", pair, interval)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var result struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	for k, v := range result.Result {
		if k == "last" {
			continue
		}
		if ohlcArr, ok := v.([]interface{}); ok {
			for _, entry := range ohlcArr {
				if entryArr, ok := entry.([]interface{}); ok && len(entryArr) > 4 {
					ts, ok1 := entryArr[0].(float64)
					closeStr, ok2 := entryArr[4].(string)
					if ok1 && ok2 {
						candleTime := time.Unix(int64(ts), 0).UTC()
						if candleTime.Equal(target) || (candleTime.Before(target) && candleTime.Add(time.Duration(interval)*time.Minute).After(target)) {
							if close, err := strconv.ParseFloat(closeStr, 64); err == nil {
								return close, nil
							}
						}
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("no price found for %s at %s", pair, target.Format(time.RFC3339))
}

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	ticker := os.Getenv("TICKER")
	if ticker == "" {
		ticker = "XBT" // Default to BTC (Kraken uses XBT for Bitcoin)
	}

	sleepSeconds := 60 // default
	if val := os.Getenv("SLEEP_SECONDS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			sleepSeconds = n
		}
	}

	changeThreshold := 10.0 // default
	if val := os.Getenv("CHANGE_THRESHOLD"); val != "" {
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			changeThreshold = n
		}
	}

	db, err := sql.Open("sqlite", "file:btc_prices.db?cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS btc_price (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		price REAL,
		timestamp DATETIME
	)`)
	if err != nil {
		panic(err)
	}

	for {
		price, err := getBTCPrice(ticker)
		if err != nil {
			panic(err)
		}
		// price = 116438.805 // Uncomment this line to test with a fixed price

		_, err = db.Exec(`INSERT INTO btc_price (price, timestamp) VALUES (?, ?)`, price, time.Now().UTC())
		if err != nil {
			panic(err)
		}

		// Get current time for output in Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			panic(err)
		}
		currentTime := time.Now().In(loc).Format("2006-01-02 15:04:05 MST")

		// Calculate and display the moving average for the last 24 hours
		row := db.QueryRow(`SELECT AVG(price) FROM btc_price WHERE timestamp >= datetime('now', '-1 day')`)
		var avg24h sql.NullFloat64
		if err := row.Scan(&avg24h); err != nil {
			panic(err)
		}

		percentChange := ((price - avg24h.Float64) / avg24h.Float64) * 100
		recommend := ""
		if percentChange > changeThreshold {
			recommend = "** SELL **"
		} else if percentChange < -changeThreshold {
			recommend = "** BUY **"
		}

		if recommend == "** SELL **" {
			beep()
		}

		p := message.NewPrinter(message.MatchLanguage("en"))
		p.Printf("[%s] %s: $%0.2f, 24h avg $%0.2f, diff %.2f%% %s\n",
			currentTime,
			ticker,
			price,
			avg24h.Float64,
			percentChange,
			recommend,
		)

		prevBuyAmount, _ := strconv.ParseFloat(os.Getenv("PREVIOUS_BUY_AMOUNT"), 64)
		prevBuyPrice, _ := strconv.ParseFloat(os.Getenv("PREVIOUS_BUY_PRICE"), 64)
		transactionFeePct, _ := strconv.ParseFloat(os.Getenv("TRANSACTION_FEE_PCT"), 64)

		prevValueUSD := prevBuyAmount * prevBuyPrice
		transactionFeeUSD := prevValueUSD * (transactionFeePct / 100)
		newValueUSD := (prevBuyAmount * price) - transactionFeeUSD
		profitUSD := newValueUSD - prevValueUSD
		if recommend == "** SELL **" {
			p.Printf("Amount %.10f, Buy Value: $%.4f, Transaction Fee: $%.4f, Sell Value: $%.4f, Profit: $%f\n",
				prevBuyAmount,
				prevValueUSD,
				transactionFeeUSD,
				newValueUSD,
				profitUSD,
			)
		}

		// Example: Get BTC price at May 22, 2025, 20:35 UTC
		if false { // Set to true to run the example
			targetTime := time.Date(2025, 5, 22, 20, 35, 0, 0, time.UTC)
			price, err := getBTCPriceAtDatetime("XXBTZUSD", 1, targetTime)
			if err != nil {
				fmt.Println("Error getting historical price:", err)
			} else {
				fmt.Printf("BTC price at %s was $%0.2f\n", targetTime.Format(time.RFC3339), price)
			}
		}

		time.Sleep(time.Duration(sleepSeconds) * time.Second)
	}
}

func beep() {
	if runtime.GOOS == "windows" {
		exec.Command("powershell", "-c", "[console]::beep(1000,300)").Run()
	} else {
		exec.Command("beep").Run()
	}
}
