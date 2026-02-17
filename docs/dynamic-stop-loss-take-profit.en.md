# Dynamic Stop Loss & Take Profit Feature

## Overview

The Dynamic Stop Loss & Take Profit feature provides flexible risk management tools for the NOFX AI trading system, supporting multiple stop loss and take profit strategies.

## Features

### 🔴 Dynamic Stop Loss

#### 1. Trailing Stop
- **Trailing Percent**: Price retracement percentage to trigger stop
- **Activation Profit**: Profit percentage to activate trailing stop
- **Use Case**: Suitable for trending markets, locks in profits while allowing price fluctuations

**Example Configuration**:
```json
{
  "enabled": true,
  "mode": "trailing",
  "trailing_percent": 2,
  "trailing_activation": 3,
  "initial_stop_percent": 3,
  "min_stop_percent": 1
}
```

**How It Works**:
1. Initial stop loss set at 3% when opening position
2. When profit reaches 3%, trailing stop is activated
3. As price rises, stop loss follows, maintaining 2% distance
4. If price retraces 2%, stop loss is triggered

#### 2. ATR-Based Stop
- **ATR Multiplier**: Stop distance = ATR × multiplier
- **ATR Period**: Period for ATR calculation (default 14)
- **Use Case**: Dynamically adjusts stop distance based on market volatility

#### 3. Support/Resistance Stop
- **Buffer Percentage**: Buffer from support/resistance levels
- **Use Case**: Sets stop loss based on key technical levels

#### 4. Time-Based Stop
- **Max Hold Hours**: Auto close after this time
- **Time Exit Profit Requirement**: Minimum profit for time-based exit (negative = allow loss)
- **Use Case**: Prevents capital from being tied up too long

---

### 🟢 Dynamic Take Profit

#### 1. Fixed Take Profit
- **Fixed Percent**: Close all at this profit percentage
- **Use Case**: Simple and direct, suitable for trades with clear targets

#### 2. Scaled Take Profit ⭐ Recommended
- **Multiple Levels**: Close positions in batches at different profit points
- **Move Stop to Breakeven**: Protect profits after reaching certain levels
- **Use Case**: Balances risk and reward, suitable for most trades

**Example Configuration**:
```json
{
  "enabled": true,
  "mode": "scaled",
  "scaled_levels": [
    {
      "profit_percent": 3,
      "close_percent": 30,
      "move_stop_to_breakeven": false
    },
    {
      "profit_percent": 5,
      "close_percent": 30,
      "move_stop_to_breakeven": true
    },
    {
      "profit_percent": 8,
      "close_percent": 40
    }
  ],
  "partial_close_enabled": true,
  "lock_profit_percent": 5
}
```

**How It Works**:
1. At 3% profit → Close 30%
2. At 5% profit → Close 30%, move stop to entry price (breakeven)
3. At 8% profit → Close remaining 40%

#### 3. ATR-Based Take Profit
- **ATR Multiplier**: Take profit distance = ATR × multiplier
- **Use Case**: Sets take profit target based on market volatility

#### 4. Resistance Take Profit
- **Resistance Buffer**: Buffer percentage from resistance levels
- **Use Case**: Sets take profit based on key technical levels

---

## Usage Guide

### Frontend Configuration

1. Open **Strategy Studio**
2. Select or create a strategy
3. Expand **Risk Control** section
4. Find **Dynamic Stop Loss** and **Dynamic Take Profit** configuration areas
5. Enable features and select appropriate modes
6. Adjust parameters
7. Save strategy

### Recommended Configurations

#### Conservative Trader
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "trailing",
    "trailing_percent": 1.5,
    "trailing_activation": 2,
    "initial_stop_percent": 2
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "scaled",
    "scaled_levels": [
      { "profit_percent": 2, "close_percent": 40, "move_stop_to_breakeven": true },
      { "profit_percent": 4, "close_percent": 30 },
      { "profit_percent": 6, "close_percent": 30 }
    ]
  }
}
```

#### Aggressive Trader
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "atr",
    "atr_multiplier": 2.5,
    "atr_period": 14
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "scaled",
    "scaled_levels": [
      { "profit_percent": 5, "close_percent": 30 },
      { "profit_percent": 10, "close_percent": 30, "move_stop_to_breakeven": true },
      { "profit_percent": 15, "close_percent": 40 }
    ]
  }
}
```

#### Day Trader
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "time_based",
    "max_hold_hours": 24,
    "time_based_exit_percent": 0,
    "initial_stop_percent": 2
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "fixed",
    "fixed_percent": 3
  }
}
```

---

## Implementation Notes

### Backend Implementation

The logic should be implemented in `trader/auto_trader.go` or `trader/position_manager.go`:

```go
func (at *AutoTrader) checkDynamicStopLoss(position *Position) (shouldClose bool, reason string) {
    config := at.strategy.RiskControl.DynamicStopLoss
    if config == nil || !config.Enabled {
        return false, ""
    }

    switch config.Mode {
    case "trailing":
        return at.checkTrailingStop(position, config)
    case "atr":
        return at.checkATRStop(position, config)
    case "support_resistance":
        return at.checkSupportResistanceStop(position, config)
    case "time_based":
        return at.checkTimeBasedStop(position, config)
    }

    return false, ""
}
```

### Position Structure Extension

Add these fields to the `Position` struct:

```go
type Position struct {
    // ... existing fields ...
    
    // Dynamic stop loss/take profit related
    HighestPrice      float64           `json:"highest_price"`
    LowestPrice       float64           `json:"lowest_price"`
    ExecutedTPLevels  map[int]bool      `json:"executed_tp_levels"`
    TrailingStopPrice float64           `json:"trailing_stop_price"`
    EntryTime         time.Time         `json:"entry_time"`
}
```

---

## Important Notes

⚠️ **Warnings**:

1. **Slippage**: Actual execution price may differ from trigger price
2. **Network Latency**: Ensure system can detect price changes in time
3. **Exchange Limitations**: Some exchanges may not support partial closes
4. **Fee Considerations**: Frequent take profits may increase trading fees
5. **Market Volatility**: Extreme market conditions may prevent execution at expected prices

---

## Future Enhancements

- [ ] Volume-based stop loss
- [ ] Funding rate-based stop loss
- [ ] Multiple take profit targets
- [ ] Conditional stop loss (e.g., stop when breaking EMA)
- [ ] AI-powered dynamic stop adjustment

---

## Related Documentation

- [Risk Control Configuration](./risk-control.md)
- [Strategy Studio Guide](./guides/strategy-studio.md)
- [Backtesting Guide](./backtest-guide.md)




