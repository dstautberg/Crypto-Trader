# Crypto-Trader

A Go application for tracking cryptocurrency prices (default: Bitcoin) using the Kraken API, with both a command-line interface and a real-time web dashboard. Features include price tracking, moving average calculations, buy/sell recommendations, and interactive charts.

## Features

### Command-Line Mode
- Fetches current price for a configurable ticker (default: BTC/XBT)
- Stores price data in a local SQLite database
- Calculates moving average and percent change
- Configurable buy/sell recommendation threshold
- Audible alert for buy/sell signals
- Profit and transaction fee calculation

### Web Dashboard Mode
- Real-time interactive price charts using Chart.js
- Live price updates every 10 seconds
- 24-hour moving average visualization
- Current price, moving average, and percentage change statistics
- Responsive modern UI with gradient design
- Runs price collection in the background automatically

## Requirements
- Go 1.24+
- SQLite3
- (Windows) PowerShell for beep alerts
- (Linux/macOS) `beep` utility for beep alerts

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd Crypto-Trader
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. (Optional) Install Air for hot reloading:
   ```bash
   go install github.com/air-verse/air@latest
   ```

## Configuration

Create a `.env` file in the project root:

```env
TICKER=BTC                # Ticker symbol (BTC, ETH, LTC, etc.)
SLEEP_SECONDS=60          # Interval between price checks (seconds)
CHANGE_THRESHOLD=10       # Percent change threshold for buy/sell
MOVING_AVG_DAYS=1         # Days for moving average calculation
PREVIOUS_BUY_AMOUNT=0.01  # Amount of crypto bought
PREVIOUS_BUY_PRICE=50000  # Price at which crypto was bought
TRANSACTION_FEE_PCT=0.2   # Transaction fee percent
```

## Usage

### Command-Line Mode
Run the price tracker in your terminal:
```bash
go run .
```

This will:
- Fetch prices at the configured interval
- Display price, moving average, and recommendations
- Show terminal-based charts
- Alert on buy/sell recommendations

### Web Dashboard Mode
Start the web server:
```bash
go run . web
```

Or use Air for hot reloading during development:
```bash
air
```

Then open your browser to: **http://localhost:8080**

The web server will:
- Start price collection in the background
- Serve the interactive dashboard
- Provide REST APIs for price data
- Auto-update charts when new data arrives

## Project Structure

```
Crypto-Trader/
├── main.go              # Main application logic and web server
├── crypto.go            # Kraken API integration
├── templates/
│   └── index.html       # Web dashboard template
├── .air.toml            # Air hot reload configuration
├── .env                 # Environment configuration
├── go.mod               # Go module dependencies
└── README.md            # This file
```

## API Endpoints

When running in web mode, the following endpoints are available:

- `GET /` - Web dashboard
- `GET /api/prices?days=1` - Historical price data with moving average
- `GET /api/latest` - Latest price and timestamp

## Development

### Hot Reloading with Air

Air automatically rebuilds and restarts your application when you save changes:

```bash
air
```

Configuration is in `.air.toml` and watches `.go`, `.html`, and template files.

### Building

Build the application:
```bash
go build -o crypto-trader
```

Run the binary:
```bash
# Command-line mode
./crypto-trader

# Web mode
./crypto-trader web
```

## Notes

- The app uses Kraken's asset codes (e.g., XBT for BTC, ETH for Ethereum)
- Database files (`*.db`, `*.db-shm`, `*.db-wal`) are stored locally
- Price data persists across restarts
- Web dashboard automatically detects new price data
- Moving average calculation uses a 24-hour rolling window

## Technologies

- **Backend**: Go with Gin web framework
- **Database**: SQLite
- **Frontend**: HTML, CSS, JavaScript with Chart.js
- **API**: Kraken REST API
- **Dev Tools**: Air for hot reloading

## License

MIT
