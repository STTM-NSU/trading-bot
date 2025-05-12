# trading-bot
Trading robot, that uses STTM index to trade on financial market.

## STTM strategy

The highest efficiency of the STTM index is achieved in the interval between trades.

Because of Effective market hypothesis (all publicly available information is immediately and fully reflected in stock market prices)
we can make decisions based on news and stocks price only when market is not working, so we can take advantage of it.

So to detect these intervals it's needed to:
- Check trading schedule for some instrument
    - Get exchange for some `instrument_id`
    - Get exchange trading schedule for provided instrument to check trades ending

In order to use the STTM index bot trades using this strategy:
- Trading bot operates with certain portfolio of instruments, so it calculates STTM indexes for each on certain interval
    - It requests STTM API for each instrument and waits till every value will be retrieved
- It selects top N percent of instruments with the highest value of the index
- Filters instrument with STTM index less than some threshold
- Then several options for each STTM top instrument is available
    - If instrument was in portfolio but in new top it didn't appear so it's need to send it with stop-loss or stop-market order
    - If instrument was in portfolio and appeared in new top then we sell it with take-profit order
        - Or if flag is provided than we don't sell this instrument and continue keep for next STTM calculation
    - Other instruments which appeared in STTM top we buy with limit or market order
    - Before selling we calculate desired amount of instruments to buy, considering:
        - Trading bot account balance
        - Proposed selling instruments price (with some sort of protection)
        - Quantity of each instrument (we can buy approximately equal amounts or growing amount - instrument with the highest index value will get the largest amount)

Like that we provide diversification (portfolio rebalancing)

At the start trading bot tries to get existing portfolio in DB but if not then it tries to make one iteration of rebalancing:
- For full-fledged work it's needed to have:
    - `T_INVEST_API_TOKEN` to communicate with broker
    - Amount of money (quantity and currency) that trading bot is allowed to use
    - Instruments list (ISIN, FIGI, ticker_classcode) with which robot will operate, or instruments type to search for the best
      through all over the market
    - Top instruments percent (for portfolio rebalancing)
    - Type of lots balancing when rebalance happens (equal or growing quantities between top STTM instruments)
    - STTM index threshold
    - STTM calculation interval (day or week (actually 5 days))
    - Technical indicators configs
    - Orders parameters:
        - Type of order when sell out gone from the index instruments: stop-loss or stop-market with percent params from current price on market
        - Behaviour on keeping in STTM top for the second time: sell with take-profit or keep until it leaves the top
        - So for take-profit it also needed to have percent parameters
        - In order to buy stocks we can use market or limit orders
            - For limit order it's needed to specify indent from current price on market not in profitable way for more likely purchase
        - It's also possible to pass percent indent from inital price for hedging and placing orders

For safety nets (all profits are achieved by diversifying using the STTM index) we use technical indicators such as:
- RSI (relative strength index) + Bollinger Bands:
    - Sell or don't buy if RSI > X and price is higher than the upper Bollinger band
    - Don't sell if RSI < Y and price is lower than the lower Bollinger band
    - Params for that indicator: X, Y (intervals for hour candles)
- EMA (exponential moving average) + MACD (moving average convergence/divergence)
    - If EMA(X) < EMA(Y) where X < Y and MACD(X, Y, signal_smoothing) < 0 then sell instruments
    - Params for that indicator: X, Y (intervals for hour candles)

## Usage

To trade with real strategy:
1. Put your config in `./configs/config.yaml`
2. Add `./configs/invest.yaml` for T-Invest API configurations
3. Provide envs through `.env` file:
```dotenv
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USERNAME=someuser
POSTGRES_PASSWORD=password
POSTGRES_DB_NAME=dbname
POSTGRES_SSL_MODE=disable
T_INVEST_API_TOKEN=ypur_token
```
4. Run `./trading-bot/main.go`

Trading bot has backtest, to run it:
1. Change `internal/config/backtest.go` BacktestCfg to your configuration
2. Run `go run ./backtest/main.go`
