# Crypto-Trader

A Go application for tracking cryptocurrency prices (default: Bitcoin) using the Kraken API, storing data in SQLite, and providing buy/sell recommendations based on configurable thresholds.

## Features
- Fetches current price for a configurable ticker (default: BTC/XBT)
- Stores price data in a local SQLite database
- Calculates 24h moving average and percent change
- Configurable buy/sell recommendation threshold
- Audible alert for buy/sell signals
- Historical price lookup for a specific date/time
- Profit and transaction fee calculation using environment variables
- All configuration via `.env` file

## Requirements
- Go 1.18+
- SQLite3
- (Windows) PowerShell for beep alerts
- (Linux/macOS) `beep` utility for beep alerts

## Environment Variables (.env)
```
TICKER=BTC                # Ticker symbol (BTC, ETH, etc.)
SLEEP_SECONDS=60          # Interval between price checks (seconds)
CHANGE_THRESHOLD=5        # Percent change threshold for buy/sell
PREVIOUS_BUY_AMOUNT=0.01  # Amount of crypto bought
PREVIOUS_BUY_PRICE=50000  # Price at which crypto was bought
TRANSACTION_FEE_PCT=0.2   # Transaction fee percent
```

## Usage
1. Clone the repository and enter the directory.
2. Copy `.env.example` to `.env` and edit as needed.
3. Run:
   ```powershell
   go run main.go
   ```
4. The app will fetch prices, store them, and print recommendations and profit/loss info.

## Notes
- The app uses Kraken's asset codes (e.g., XBT for BTC).
- Database files (`*.db`, `*.db-shm`, `*.db-wal`) are ignored by git.
- To get a historical price for a specific date/time, see the example in `main.go`.

## License
MIT
