import csv
import datetime
import os
import sys
import time
import requests


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
            print("✗ No cash available to buy")
            return

        quantity = (self.cash * (1 - self.fee_rate)) / price
        self.holdings += quantity
        self.cash = 0.0
        self.entry_price = price
        self.log_event("BUY", price, quantity)
        print(f"✓ Bought {quantity:.8f} {self.ticker} at ${price:.5f}")

    def sell_ladder(self, price):
        """Execute ladder sell strategy: sell 25% at each price level."""
        if price <= 0:
            raise ValueError("Price must be positive")
        if self.holdings <= 0:
            print(f"✗ No {self.ticker} holdings to sell")
            return

        price_gain_percent = ((price - self.entry_price) / self.entry_price) * 100
        
        # Sell 25% at +5%, +10%, +15%, +20% gains
        if price_gain_percent >= 5.0:
            self.sell_partial(price, 0.25)
        elif price_gain_percent >= 10.0:
            self.sell_partial(price, 0.25)
        elif price_gain_percent >= 15.0:
            self.sell_partial(price, 0.25)
        elif price_gain_percent >= 20.0:
            self.sell_partial(price, 0.25)

    def sell_partial(self, price, percent):
        """Sell a percentage of holdings at current price."""
        if price <= 0:
            raise ValueError("Price must be positive")
        if not 0 < percent <= 1:
            raise ValueError("Percent must be between 0 and 1")
        if self.holdings <= 0:
            print(f"✗ No {self.ticker} holdings to sell")
            return

        quantity = self.holdings * percent
        revenue = quantity * price * (1 - self.fee_rate)
        self.cash += revenue
        self.holdings -= quantity
        self.log_event(f"SELL_{int(percent * 100)}%", price, quantity)
        print(f"✓ Sold {quantity:.8f} {self.ticker} at ${price:.5f} for ${revenue:.2f}")

    def get_status(self, current_price):
        """Print current portfolio status."""
        portfolio_value = self._get_portfolio_value(current_price)
        profit_loss = portfolio_value - self.initial_cash
        profit_percent = (profit_loss / self.initial_cash * 100) if self.initial_cash > 0 else 0

        print(f"\n📊 Portfolio Status ({self.ticker}):")
        print(f"   Cash: ${self.cash:.2f}")
        print(f"   {self.ticker} Holdings: {self.holdings:.8f}")
        print(f"   Portfolio Value: ${portfolio_value:.2f}")
        print(f"   Profit/Loss: ${profit_loss:.2f} ({profit_percent:+.2f}%)\n")


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
            print(f"✗ Kraken API error: {data['error']}")
            return None
        
        if data.get('result'):
            # Get the ticker data (first/only pair in result)
            ticker_data = list(data['result'].values())[0]
            # 'c' is the close price, [0] is the current price
            price = float(ticker_data['c'][0])
            return price
        else:
            print("✗ No result from Kraken API")
    except requests.RequestException as e:
        print(f"✗ Error fetching {ticker} price from Kraken: {e}")
    except (KeyError, IndexError, ValueError) as e:
        print(f"✗ Error parsing Kraken response: {e}")
    return None


def print_available_tickers():
    """Print available tickers and their current prices."""
    tickers = ["GALA", "SAND", "MANA", "ENJ", "BEAM"]
    print("📊 Available Tickers and Current Prices:")
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
    Uses a laddered exit strategy.
    
    Args:
        initial_cash: Starting portfolio value in USD
        ticker: Cryptocurrency ticker symbol (e.g., GALA, SAND, MANA)
        interval: Seconds to wait between each iteration
    """
    sim = CryptoSim(initial_cash, ticker)
    iteration = 0
    prev_price = None

    print(f"🚀 Starting {ticker} trading simulation with ${initial_cash} USD")
    print(f"📊 Using Kraken API for real-time prices (USD pairs)")
    print(f"📈 Strategy: Laddered exit (sell 25% at +5%, +10%, +15%, +20% gains)")
    print(f"Press Ctrl+C to stop\n")

    try:
        while True:
            # Fetch current price from Kraken
            price = get_crypto_price(ticker)

            if price is None:
                print(f"⚠️  Failed to fetch {ticker} price, retrying in 60 seconds...")
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
                
                # Sell ladder based on gains from entry price
                if sim.holdings > 0:
                    sim.sell_ladder(price)

            sim.get_status(price)
            prev_price = price
            iteration += 1
            time.sleep(interval)

    except KeyboardInterrupt:
        print(f"\n⏹️  Simulation stopped")
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