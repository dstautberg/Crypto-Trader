import base64
import csv
import datetime
import hashlib
import hmac
import os
import sys
import time
import urllib.parse
import requests

# UI Icons
ICON_ROCKET     = "\U0001F680"  # 🚀
ICON_CHART_BAR  = "\U0001F4CA"  # 📊
ICON_CHART_UP   = "\U0001F4C8"  # 📈
ICON_CHECK      = "\u2713"      # ✓
ICON_CROSS      = "\u2717"      # ✗
ICON_WARNING    = "\u26A0\uFE0F"  # ⚠️
ICON_STOP       = "\u23F9\uFE0F"  # ⏹️
ICON_UP_ARROW   = "\u2191"      # ↑
ICON_DOWN_ARROW = "\u2193"      # ↓
ICON_DOT        = "\u25CF"      # ●

# Set KRAKEN_LIVE_TRADING=1 to execute real orders; default is virtual/simulation only
LIVE_TRADING = os.environ.get("KRAKEN_LIVE_TRADING", "0") == "1"
KRAKEN_API_KEY = os.environ.get("KRAKEN_API_KEY", "")
KRAKEN_API_SECRET = os.environ.get("KRAKEN_API_SECRET", "")


def _kraken_signature(url_path, data, secret):
    """Generate HMAC-SHA512 signature for Kraken private API calls."""
    encoded = (str(data["nonce"]) + urllib.parse.urlencode(data)).encode()
    message = url_path.encode() + hashlib.sha256(encoded).digest()
    mac = hmac.new(base64.b64decode(secret), message, hashlib.sha512)
    return base64.b64encode(mac.digest()).decode()


def kraken_place_order(pair, side, volume):
    """
    Place a market order on Kraken. Only called when KRAKEN_LIVE_TRADING=1.

    Args:
        pair:   Kraken pair string, e.g. "GALAUSD"
        side:   "buy" or "sell"
        volume: Quantity of base currency to trade

    Returns:
        Transaction ID string on success, None on failure.
    """
    if not LIVE_TRADING:
        return None

    if not KRAKEN_API_KEY or not KRAKEN_API_SECRET:
        print(f"{ICON_WARNING}  Live trading enabled but KRAKEN_API_KEY / KRAKEN_API_SECRET not set")
        return None

    url_path = "/0/private/AddOrder"
    url = f"https://api.kraken.com{url_path}"
    data = {
        "nonce": str(int(time.time() * 1000)),
        "ordertype": "market",
        "type": side,
        "volume": f"{volume:.8f}",
        "pair": pair,
    }
    headers = {
        "API-Key": KRAKEN_API_KEY,
        "API-Sign": _kraken_signature(url_path, data, KRAKEN_API_SECRET),
    }

    try:
        response = requests.post(url, headers=headers, data=data, timeout=10)
        response.raise_for_status()
        result = response.json()

        if result.get("error"):
            print(f"{ICON_CROSS} Kraken order error: {result['error']}")
            return None

        txid = result["result"]["txid"][0]
        print(f"{ICON_CHECK} Live order placed: {side.upper()} {volume:.8f} {pair} | txid: {txid}")
        return txid

    except requests.RequestException as e:
        print(f"{ICON_CROSS} Failed to place live order: {e}")
        return None


class CryptoSim:
    """Simulates cryptocurrency trading with virtual portfolio tracking."""

    def __init__(self, initial_cash=100.0, ticker="GALA"):
        self.initial_cash = initial_cash
        self.cash = initial_cash
        self.holdings = 0.0
        self.entry_price = 0.0
        self.fee_rate = 0.001  # 0.1% exchange fee
        self.ticker = ticker.upper()
        self.log_file = f"{self.ticker.lower()}_trade_log.csv"
        self.price_history = []  # Track prices and trades
        self.sold_tiers = set()  # Track which gain tiers have been sold
        self._init_csv()

    def _init_csv(self):
        """Create CSV file with headers if it doesn't exist."""
        if not os.path.exists(self.log_file):
            with open(self.log_file, 'w', newline='') as f:
                writer = csv.writer(f)
                writer.writerow([
                    "Timestamp", "Action", "Price", "Quantity",
                    "Cash", "Holdings", "Portfolio_Value"
                ])

    def _get_portfolio_value(self, current_price):
        """Calculate total portfolio value at current price."""
        return self.cash + (self.holdings * current_price)

    def log_event(self, action, price, quantity):
        """Log trade event to CSV file."""
        portfolio_value = self._get_portfolio_value(price)
        with open(self.log_file, 'a', newline='') as f:
            writer = csv.writer(f)
            writer.writerow([
                datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                action,
                f"{price:.5f}",
                f"{quantity:.8f}",
                f"{self.cash:.2f}",
                f"{self.holdings:.8f}",
                f"{portfolio_value:.2f}"
            ])

    def buy(self, price):
        """Execute market buy using all available cash."""
        if price <= 0:
            raise ValueError("Price must be positive")
        if self.cash <= 0:
            print(f"{ICON_CROSS} No cash available to buy")
            return

        quantity = (self.cash * (1 - self.fee_rate)) / price
        self.holdings += quantity
        self.cash = 0.0
        self.entry_price = price
        self.sold_tiers = set()  # Reset tiers for new position
        self.log_event("BUY", price, quantity)
        self.price_history.append({"price": price, "action": "BUY"})
        print(f"{ICON_CHECK} Bought {quantity:.8f} {self.ticker} at ${price:.5f}")

        kraken_place_order(f"{self.ticker}USD", "buy", quantity)

    def sell_ladder(self, price):
        """Execute ladder sell strategy: sell 25% at each price level."""
        if price <= 0:
            raise ValueError("Price must be positive")
        if self.holdings <= 0:
            return

        price_gain_percent = ((price - self.entry_price) / self.entry_price) * 100

        # Check tiers ascending - trigger the lowest unmet tier and wait for next iteration
        for tier in [5, 10, 15, 20]:
            if price_gain_percent >= tier and tier not in self.sold_tiers:
                self.sell_partial(price, 0.25)
                self.sold_tiers.add(tier)
                break  # One tier per iteration

    def sell_partial(self, price, percent):
        """Sell a percentage of holdings at current price."""
        if price <= 0:
            raise ValueError("Price must be positive")
        if not 0 < percent <= 1:
            raise ValueError("Percent must be between 0 and 1")
        if self.holdings <= 0:
            return

        quantity = self.holdings * percent
        revenue = quantity * price * (1 - self.fee_rate)
        self.cash += revenue
        self.holdings -= quantity
        self.log_event(f"SELL_{int(percent * 100)}%", price, quantity)
        self.price_history.append({"price": price, "action": "SELL"})
        print(f"{ICON_CHECK} Sold {quantity:.8f} {self.ticker} at ${price:.5f} for ${revenue:.2f}")

        kraken_place_order(f"{self.ticker}USD", "sell", quantity)

    def get_status(self, current_price):
        """Print current portfolio status with price chart."""
        portfolio_value = self._get_portfolio_value(current_price)
        profit_loss = portfolio_value - self.initial_cash
        profit_percent = (profit_loss / self.initial_cash * 100) if self.initial_cash > 0 else 0

        print(f"\n{ICON_CHART_BAR} Portfolio Status ({self.ticker}):")
        print(f"   Cash: ${self.cash:.2f}")
        print(f"   {self.ticker} Holdings: {self.holdings:.8f}")
        print(f"   Portfolio Value: ${portfolio_value:.2f}")
        print(f"   Profit/Loss: ${profit_loss:.2f} ({profit_percent:+.2f}%)")

        # Print mini price chart
        self._print_price_chart(current_price)
        print()

    def _print_price_chart(self, current_price):
        """Print a simple text-based price chart with buy/sell markers."""
        if len(self.price_history) < 2:
            return

        # Keep last 20 prices for chart
        recent_history = self.price_history[-20:]

        if len(recent_history) < 2:
            return

        prices = [p["price"] for p in recent_history]
        actions = [p["action"] for p in recent_history]

        min_price = min(prices)
        max_price = max(prices)
        price_range = max_price - min_price

        if price_range == 0:
            return

        print(f"   {ICON_CHART_UP} Price Chart (last {len(recent_history)} updates):")

        # Normalize prices to 0-9 scale
        chart_height = 10
        for level in range(chart_height - 1, -1, -1):
            level_price = min_price + (price_range / chart_height) * (level + 1)
            line = f"   {level_price:10.7f} \u2502 "

            for i, price in enumerate(prices):
                normalized = (price - min_price) / price_range
                price_level = int(normalized * (chart_height - 1))

                if price_level == level:
                    if actions[i] == "BUY":
                        line += ICON_UP_ARROW + " "
                    elif actions[i] == "SELL":
                        line += ICON_DOWN_ARROW + " "
                    else:
                        line += ICON_DOT + " "
                else:
                    line += "  "

            print(line)

        # Bottom axis
        print(f"   {min_price:10.7f} \u2514" + "\u2500" * (len(prices) * 2))


def get_crypto_price(ticker):
    """Fetch current cryptocurrency price from Kraken API (USD pairs)."""
    try:
        # Kraken pair format: TICKER + USD (e.g., GALAUSD, SANDUSD)
        pair = f"{ticker.upper()}USD"
        url = f"https://api.kraken.com/0/public/Ticker?pair={pair}"
        response = requests.get(url, timeout=5)
        response.raise_for_status()
        data = response.json()

        # Check for errors in response
        if data.get('error'):
            print(f"{ICON_CROSS} Kraken API error: {data['error']}")
            return None

        if data.get('result'):
            # Get the ticker data (first/only pair in result)
            ticker_data = list(data['result'].values())[0]
            # 'c' is the close price, [0] is the current price
            price = float(ticker_data['c'][0])
            return price
        else:
            print(f"{ICON_CROSS} No result from Kraken API")
    except requests.RequestException as e:
        print(f"{ICON_CROSS} Error fetching {ticker} price from Kraken: {e}")
    except (KeyError, IndexError, ValueError) as e:
        print(f"{ICON_CROSS} Error parsing Kraken response: {e}")
    return None


def print_available_tickers():
    """Print available tickers and their current prices."""
    tickers = ["GALA", "SAND", "MANA", "ENJ", "BEAM"]
    print(f"{ICON_CHART_BAR} Available Tickers and Current Prices:")
    print("-" * 50)

    for ticker in tickers:
        price = get_crypto_price(ticker)
        if price is not None:
            print(f"   {ticker:6} ${price:.5f}")
        else:
            print(f"   {ticker:6} [Failed to fetch]")
    print("-" * 50 + "\n")


def simulate_trading_loop(initial_cash=100.0, ticker="GALA", interval=60):
    """
    Run a simulated trading loop with real cryptocurrency prices from Kraken (USD pairs).
    Uses a laddered exit strategy with visual price chart.

    Args:
        initial_cash: Starting portfolio value in USD
        ticker: Cryptocurrency ticker symbol (e.g., GALA, SAND, MANA)
        interval: Seconds to wait between each iteration
    """
    sim = CryptoSim(initial_cash, ticker)
    iteration = 0
    prev_price = None

    mode = "LIVE TRADING" if LIVE_TRADING else "virtual simulation"
    print(f"{ICON_ROCKET} Starting {ticker} trading in {mode} with ${initial_cash} USD")
    print(f"{ICON_CHART_BAR} Using Kraken API for real-time prices (USD pairs)")
    print(f"{ICON_CHART_UP} Strategy: Laddered exit (sell 25% at +5%, +10%, +15%, +20% gains)")
    print(f"Chart: {ICON_UP_ARROW} = BUY, {ICON_DOWN_ARROW} = SELL, {ICON_DOT} = price only")
    if LIVE_TRADING:
        print(f"{ICON_WARNING}  LIVE TRADING ACTIVE - real orders will be placed on Kraken")
    print(f"Press Ctrl+C to stop\n")

    try:
        while True:
            # Fetch current price from Kraken
            price = get_crypto_price(ticker)

            if price is None:
                print(f"{ICON_WARNING}  Failed to fetch {ticker} price, retrying in 60 seconds...")
                time.sleep(60)
                continue

            print(f"[Iteration {iteration + 1}] Current {ticker} Price: ${price:.5f}")

            # Laddered exit strategy
            if iteration == 0:
                sim.buy(price)
            elif prev_price is not None:
                price_change = ((price - prev_price) / prev_price) * 100

                # Buy if price drops >2%
                if price_change < -2.0 and sim.cash > 0:
                    sim.buy(price)

                # Always track price for chart
                sim.price_history.append({"price": price, "action": None})

                # Sell ladder based on gains from entry price
                if sim.holdings > 0:
                    sim.sell_ladder(price)

            sim.get_status(price)
            prev_price = price
            iteration += 1
            time.sleep(interval)

    except KeyboardInterrupt:
        print(f"\n{ICON_STOP}  Simulation stopped")
        if prev_price:
            sim.get_status(prev_price)


def main():
    """Main entry point."""
    # Print available tickers
    print_available_tickers()

    # Get ticker from command line argument or default to GALA
    ticker = sys.argv[1].upper() if len(sys.argv) > 1 else "GALA"
    initial_cash = float(sys.argv[2]) if len(sys.argv) > 2 else 100.0
    interval = int(sys.argv[3]) if len(sys.argv) > 3 else 60

    print(f"Selected Ticker: {ticker}")
    print(f"Initial Cash: ${initial_cash} USD")
    print(f"Update Interval: {interval} seconds\n")

    simulate_trading_loop(initial_cash, ticker, interval)


if __name__ == "__main__":
    main()
