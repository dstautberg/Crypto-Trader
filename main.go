package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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

	body, err := io.ReadAll(resp.Body)
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

	// Read moving average days from .env
	movingAvgDays := 1 // default to 1 day
	if val := os.Getenv("MOVING_AVG_DAYS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			movingAvgDays = n
		}
	}

	for {
		price, err := getBTCPrice(ticker)
		if err != nil {
			fmt.Println("Error fetching price:", err)
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
			continue
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

		// Calculate and display the moving average for the last N days
		query := fmt.Sprintf(`SELECT AVG(price) FROM btc_price WHERE timestamp >= datetime('now', '-%d day')`, movingAvgDays)
		row := db.QueryRow(query)
		var avgNDays sql.NullFloat64
		if err := row.Scan(&avgNDays); err != nil {
			panic(err)
		}

		percentChange := ((price - avgNDays.Float64) / avgNDays.Float64) * 100
		recommend := ""
		if percentChange > changeThreshold {
			recommend = "** SELL **"
		} else if percentChange < -changeThreshold {
			recommend = "** BUY **"
		}

		if recommend == "** SELL **" {
			beep()
		}

		prevBuyAmount, _ := strconv.ParseFloat(os.Getenv("PREVIOUS_BUY_AMOUNT"), 64)
		prevBuyPrice, _ := strconv.ParseFloat(os.Getenv("PREVIOUS_BUY_PRICE"), 64)
		transactionFeePct, _ := strconv.ParseFloat(os.Getenv("TRANSACTION_FEE_PCT"), 64)

		prevValueUSD := prevBuyAmount * prevBuyPrice
		transactionFeeUSD := prevValueUSD * (transactionFeePct / 100)
		newValueUSD := (prevBuyAmount * price) - transactionFeeUSD
		profitUSD := newValueUSD - prevValueUSD

		p := message.NewPrinter(message.MatchLanguage("en"))
		line := "\u2500"
		p.Println(strings.Repeat(line, 105))
		p.Printf("%s - %s $%0.2f, %dd avg $%0.2f, diff %.2f%% %s\n",
			currentTime,
			ticker,
			price,
			movingAvgDays,
			avgNDays.Float64,
			percentChange,
			recommend,
		)
		if recommend == "** SELL **" {
			p.Printf("Amount %.10f, Buy Value: $%.4f, Transaction Fee: $%.4f, Sell Value: $%.4f, Profit: $%f\n",
				prevBuyAmount,
				prevValueUSD,
				transactionFeeUSD,
				newValueUSD,
				profitUSD,
			)
		}
		p.Println(strings.Repeat(line, 105))

		// Common Unicode circle types:
		// Small Circle:      ◦ (U+25E6)  \u25E6
		// Medium Circle:     ○ (U+25CB)  \u25CB
		// Large Circle:      ● (U+25CF)  \u25CF
		// White Circle:      ◯ (U+25EF)  \u25EF
		// Black Circle:      ● (U+25CF)  \u25CF
		// Dotted Circle:     ◌ (U+25CC)  \u25CC
		// Bullseye/Fisheye:  ◉ (U+25C9)  \u25C9
		// Circle Vert Fill:  ◍ (U+25CD)  \u25CD
		// "Bullet" • (U+2022) — a small filled circle, commonly used for lists.
		// "Black Small Circle" ● (U+25CF) — but this is the same as the large filled circle.
		// "One Dot Leader" ․ (U+2024) — very small, but not visually circular in all fonts.
		// "Middle Dot" · (U+00B7) — a small centered dot, but not a perfect circle.
		// You can use these as needed for chart points.
		circle := "\u2022"
		fmt.Printf("Price: \x1b[37m%s\x1b[0m %dd MA: \x1b[32m%s\x1b[0m Both: \x1b[33m%s\x1b[0m",
			circle, movingAvgDays, circle, circle,
		)

		// Inline chart display after price output
		// Query prices and timestamps for the last N days (matching moving average)
		queryChart := fmt.Sprintf(`SELECT price, timestamp FROM btc_price WHERE timestamp >= datetime('now', '-%d day') ORDER BY timestamp`, movingAvgDays)
		rows, err := db.Query(queryChart)
		if err == nil {
			defer rows.Close()
			var prices []float64
			for rows.Next() {
				var p float64
				var t string
				if err := rows.Scan(&p, &t); err == nil {
					prices = append(prices, p)
				}
			}
			if len(prices) > 0 {
				min, max := prices[0], prices[0]
				high, low := prices[0], prices[0]
				for _, p := range prices {
					if p < min {
						min = p
					}
					if p > max {
						max = p
					}
					if p < low {
						low = p
					}
					if p > high {
						high = p
					}
				}
				chartWidth := 100
				chartHeight := 20
				step := 1
				if len(prices) > chartWidth {
					step = len(prices) / chartWidth
				}
				chartData := make([]float64, 0, chartWidth)
				for i := 0; i < len(prices); i += step {
					chartData = append(chartData, prices[i])
				}
				ma := make([]float64, len(prices))
				window := 24 * 60 / step
				if window < 1 {
					window = 1
				}
				for i := range prices {
					start := i - window + 1
					if start < 0 {
						start = 0
					}
					sum := 0.0
					for j := start; j <= i; j++ {
						sum += prices[j]
					}
					ma[i] = sum / float64(i-start+1)
				}
				maChartData := make([]float64, 0, chartWidth)
				for i := 0; i < len(ma); i += step {
					maChartData = append(maChartData, ma[i])
				}
				// Print chart to console
				for y := chartHeight - 1; y >= 0; y-- {
					for x := 0; x < len(chartData); x++ {
						priceNorm := (chartData[x] - min) / (max - min)
						priceLevel := int(priceNorm * float64(chartHeight-1))
						maNorm := (maChartData[x] - min) / (max - min)
						maLevel := int(maNorm * float64(chartHeight-1))
						if priceLevel == y && maLevel == y {
							p.Printf("\x1b[33m%s\x1b[0m", circle) // yellow
						} else if priceLevel == y {
							p.Printf("\x1b[37m%s\x1b[0m", circle) // white
						} else if maLevel == y {
							p.Printf("\x1b[32m%s\x1b[0m", line) // green
						} else {
							p.Print(" ")
						}
					}
					fmt.Println()
				}
				p.Printf("\n")
			}
		}
		p.Println(strings.Repeat(line, 105), "\n")

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
