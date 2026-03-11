import ccxt
import pandas as pd

def calculate_kraken_wma(symbol='BTC/USD', timeframe='1h', period=14):
    # 'enableRateLimit' is good practice for automated scripts
    exchange = ccxt.kraken({'enableRateLimit': True})
    
    try:
        # Fetch OHLCV data (Open, High, Low, Close, Volume)
        # Kraken returns ~720 data points by default
        bars = exchange.fetch_ohlcv(symbol, timeframe=timeframe, limit=period * 2)
        df = pd.DataFrame(bars, columns=['ts', 'open', 'high', 'low', 'close', 'volume'])
        
        # Calculate Linear Weighted Moving Average
        weights = list(range(1, period + 1))
        def wma_calc(prices):
            return (prices * weights).sum() / sum(weights)

        df['WMA'] = df['close'].rolling(window=period).apply(wma_calc, raw=True)
        
        last_price = df['close'].iloc[-1]
        last_wma = df['WMA'].iloc[-1]
        
        return last_price, last_wma
    except Exception as e:
        print(f"Error fetching data from Kraken: {e}")
        return None, None

if __name__ == "__main__":
    # Run the calculation
    price, wma = calculate_kraken_wma()
    if price:
        print(f"BTC/USD Price: ${price:,.2f} | 14-period WMA: ${wma:,.2f}")
        print("Trend: " + ("Bullish" if price > wma else "Bearish"))
