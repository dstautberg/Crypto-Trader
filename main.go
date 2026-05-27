package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/text/message"

	_ "modernc.org/sqlite"
)

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	// Check if web server mode is requested
	if len(os.Args) > 1 && os.Args[1] == "web" {
		webServer()
		return
	}

	// Run console mode
	consoleMode()
}

func consoleMode() {
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

	// Create settings table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		initial_funds REAL DEFAULT 0,
		transaction_fee_rate REAL DEFAULT 1.0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(err)
	}

	// Create trading signals table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS trading_signals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		price_id INTEGER,
		action TEXT,
		price REAL,
		timestamp DATETIME,
		FOREIGN KEY(price_id) REFERENCES btc_price(id)
	)`)
	if err != nil {
		panic(err)
	}

	runPriceCollection(db, true)
}

func runPriceCollection(db *sql.DB, showConsoleOutput bool) {
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

	// Read moving average days from .env
	movingAvgDays := 1 // default to 1 day
	if val := os.Getenv("MOVING_AVG_DAYS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			movingAvgDays = n
		}
	}

	for {
		price, usedURL, err := getBTCPrice(ticker)
		if err != nil {
			fmt.Println("Error fetching price:", err, "url:", usedURL)
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
			continue
		}
		// price = 116438.805 // Uncomment this line to test with a fixed price

		result, err := db.Exec(`INSERT INTO btc_price (price, timestamp) VALUES (?, ?)`, price, time.Now().UTC())
		if err != nil {
			panic(err)
		}

		priceID, _ := result.LastInsertId()

		// Call the trading algorithm to analyze the price
		signal, err := TradingAlgorithm(db, price, movingAvgDays, changeThreshold)
		if err != nil {
			fmt.Println("Error running trading algorithm:", err)
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
			continue
		}

		// Record trading signal if action is BUY or SELL
		if signal.Action == "BUY" || signal.Action == "SELL" {
			_, err := db.Exec(`INSERT INTO trading_signals (price_id, action, price, timestamp) VALUES (?, ?, ?, ?)`,
				priceID, signal.Action, price, time.Now().UTC())
			if err != nil {
				fmt.Println("Error recording trading signal:", err)
			}
		}

		// Get current time for output in Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			panic(err)
		}
		currentTime := time.Now().In(loc).Format("2006-01-02 15:04:05 MST")

		// Use signal data from algorithm
		percentChange := signal.PercentChange
		recommend := signal.Recommendation
		avgNDays := sql.NullFloat64{Float64: signal.MovingAverage, Valid: signal.MovingAverage > 0}

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

		if showConsoleOutput {
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

			circle := "\u2022"
			fmt.Printf("Price: \x1b[37m%s\x1b[0m %dd MA: \x1b[32m%s\x1b[0m Both: \x1b[33m%s\x1b[0m",
				circle, movingAvgDays, circle, circle,
			)

			// Inline chart display after price output
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
		} else {
			// Just log price collection for web mode
			fmt.Printf("[%s] Price collected: $%.2f\n", time.Now().Format("15:04:05"), price)
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

func webServer() {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Open database connection
	db, err := sql.Open("sqlite", "file:btc_prices.db?cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS btc_price (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		price REAL,
		timestamp DATETIME
	)`)
	if err != nil {
		panic(err)
	}

	// Create settings table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		initial_funds REAL DEFAULT 0,
		transaction_fee_rate REAL DEFAULT 1.0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(err)
	}

	// Create trading signals table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS trading_signals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		price_id INTEGER,
		action TEXT,
		price REAL,
		timestamp DATETIME,
		FOREIGN KEY(price_id) REFERENCES btc_price(id)
	)`)
	if err != nil {
		panic(err)
	}

	// Start price collection in background
	go runPriceCollection(db, false)

	// Create a new Gin router
	router := gin.Default()

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	// Define the home route
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// API endpoint to get historical price data
	router.GET("/api/prices", func(c *gin.Context) {
		daysStr := c.DefaultQuery("days", "1")
		daysInt, _ := strconv.Atoi(daysStr)
		if daysInt < 1 {
			daysInt = 1
		}

		query := fmt.Sprintf(`SELECT id, price, timestamp FROM btc_price WHERE timestamp >= datetime('now', '-%d day') ORDER BY timestamp`, daysInt)
		rows, err := db.Query(query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type PricePoint struct {
			ID        int     `json:"id"`
			Price     float64 `json:"price"`
			Timestamp string  `json:"timestamp"`
		}

		type Signal struct {
			Index  int     `json:"index"`
			Action string  `json:"action"`
			Price  float64 `json:"price"`
		}

		var prices []PricePoint
		var rawPrices []float64
		for rows.Next() {
			var p PricePoint
			if err := rows.Scan(&p.ID, &p.Price, &p.Timestamp); err == nil {
				prices = append(prices, p)
				rawPrices = append(rawPrices, p.Price)
			}
		}

		// Fetch trading signals for the same time range
		signalQuery := fmt.Sprintf(`SELECT ts.action, ts.price, ts.price_id FROM trading_signals ts 
			WHERE ts.timestamp >= datetime('now', '-%d day') ORDER BY ts.timestamp`, daysInt)
		signalRows, err := db.Query(signalQuery)
		var signals []Signal
		if err == nil {
			defer signalRows.Close()
			for signalRows.Next() {
				var action string
				var price float64
				var priceID int
				if err := signalRows.Scan(&action, &price, &priceID); err == nil {
					// Find matching index in prices array by price ID
					for i, p := range prices {
						if p.ID == priceID {
							signals = append(signals, Signal{
								Index:  i,
								Action: action,
								Price:  price,
							})
							break
						}
					}
				}
			}
		}

		// Estimate samples per day from returned data so we can compute window sizes for day-based WMAs
		samplesPerDay := 1.0
		if len(rawPrices) > 0 {
			samplesPerDay = float64(len(rawPrices)) / float64(daysInt)
			if samplesPerDay < 1.0 {
				samplesPerDay = 1.0
			}
		}

		// Window sizes in samples for 7-day and 30-day WMAs
		window7 := int(samplesPerDay * 7.0)
		window30 := int(samplesPerDay * 30.0)
		if window7 < 1 {
			window7 = 1
		}
		if window30 < 1 {
			window30 = 1
		}
		if window7 > len(rawPrices) {
			window7 = len(rawPrices)
		}
		if window30 > len(rawPrices) {
			window30 = len(rawPrices)
		}

		// Compute WMAs (arrays aligned with prices)
		var wma7, wma30 []float64
		if len(rawPrices) > 0 {
			wma7 = computeWMA(rawPrices, window7)
			wma30 = computeWMA(rawPrices, window30)
		} else {
			wma7 = []float64{}
			wma30 = []float64{}
		}

		c.JSON(http.StatusOK, gin.H{
			"prices":  prices,
			"wma7":    wma7,
			"wma30":   wma30,
			"signals": signals,
		})
	})

	// API endpoint to get latest price
	router.GET("/api/latest", func(c *gin.Context) {
		row := db.QueryRow(`SELECT price, timestamp FROM btc_price ORDER BY id DESC LIMIT 1`)
		var price float64
		var timestamp string
		if err := row.Scan(&price, &timestamp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"price":     price,
			"timestamp": timestamp,
		})
	})

	// API endpoint to get settings
	router.GET("/api/settings", func(c *gin.Context) {
		row := db.QueryRow(`SELECT initial_funds, transaction_fee_rate FROM settings ORDER BY id DESC LIMIT 1`)
		var initialFunds float64
		var transactionFeeRate float64
		if err := row.Scan(&initialFunds, &transactionFeeRate); err != nil {
			if err == sql.ErrNoRows {
				// No settings yet, return defaults
				c.JSON(http.StatusOK, gin.H{
					"initial_funds":        0,
					"transaction_fee_rate": 1.0,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"initial_funds":        initialFunds,
			"transaction_fee_rate": transactionFeeRate,
		})
	})

	// API endpoint to save settings
	router.POST("/api/settings", func(c *gin.Context) {
		var input struct {
			InitialFunds       float64 `json:"initial_funds"`
			TransactionFeeRate float64 `json:"transaction_fee_rate"`
		}
		if err := c.BindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Insert new settings record
		_, err := db.Exec(`INSERT INTO settings (initial_funds, transaction_fee_rate, updated_at) VALUES (?, ?, ?)`,
			input.InitialFunds, input.TransactionFeeRate, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":              "Settings saved successfully",
			"initial_funds":        input.InitialFunds,
			"transaction_fee_rate": input.TransactionFeeRate,
		})
	})

	// Start the server on port 8080
	fmt.Println("Starting web server on http://localhost:8080")
	router.Run(":8080")
}
