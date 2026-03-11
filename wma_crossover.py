import ccxt
import pandas as pd

def get_crossover_signal(symbol='BTC/USD', fast_p=7, slow_p=30):
    exchange = ccxt.kraken({'enableRateLimit': True})
    
    # Fetch enough data to cover the slow period
    bars = exchange.fetch_ohlcv(symbol, timeframe='1d', limit=slow_p + 10)
    df = pd.DataFrame(bars, columns=['ts', 'open', 'high', 'low', 'close', 'volume'])

    def calculate_wma(prices, period):
        weights = list(range(1, period + 1))
        return (prices[-period:] * weights).sum() / sum(weights)

    # Calculate both WMAs
    fast_wma = calculate_wma(df['close'], fast_p)
    slow_wma = calculate_wma(df['close'], slow_p)
    current_price = df['close'].iloc[-1]

    # Signal Logic
    if fast_wma > slow_wma:
        signal = "🚀 BULLISH (Golden Cross)"
    elif fast_wma < slow_wma:
        signal = "⚠️ BEARISH (Death Cross)"
    else:
        signal = "NEUTRAL"

    return current_price, fast_wma, slow_wma, signal

if __name__ == "__main__":
    price, fast, slow, signal = get_crossover_signal()
    print(f"Price: ${price:,.2f} | Fast WMA: {fast:.2f} | Slow WMA: {slow:.2f}")
    print(f"Signal: {signal}")
