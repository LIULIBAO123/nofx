package backtest

import (
	"fmt"
	"sort"
	"time"

	"nofx/market"
)

type timeframeSeries struct {
	klines     []market.Kline
	closeTimes []int64
}

type symbolSeries struct {
	byTF map[string]*timeframeSeries
}

// DataFeed manages historical kline data and provides time-progressive snapshots for backtesting.
type DataFeed struct {
	cfg           BacktestConfig
	symbols       []string
	timeframes    []string
	symbolSeries  map[string]*symbolSeries
	decisionTimes []int64
	primaryTF     string
	longerTF      string
}

func NewDataFeed(cfg BacktestConfig) (*DataFeed, error) {
	df := &DataFeed{
		cfg:          cfg,
		symbols:      make([]string, len(cfg.Symbols)),
		timeframes:   append([]string(nil), cfg.Timeframes...),
		symbolSeries: make(map[string]*symbolSeries),
		primaryTF:    cfg.DecisionTimeframe,
	}
	copy(df.symbols, cfg.Symbols)

	if err := df.loadAll(); err != nil {
		return nil, err
	}

	return df, nil
}

func (df *DataFeed) loadAll() error {
	start := time.Unix(df.cfg.StartTS, 0)
	end := time.Unix(df.cfg.EndTS, 0)

	// longest timeframe used for auxiliary indicators
	var longestDur time.Duration
	for _, tf := range df.timeframes {
		dur, err := market.TFDuration(tf)
		if err != nil {
			return err
		}
		if dur > longestDur {
			longestDur = dur
			df.longerTF = tf
		}
	}

	for _, symbol := range df.symbols {
		ss := &symbolSeries{byTF: make(map[string]*timeframeSeries)}
		for _, tf := range df.timeframes {
			dur, _ := market.TFDuration(tf)
			buffer := dur * 200
			fetchStart := start.Add(-buffer)
			if fetchStart.Before(time.Unix(0, 0)) {
				fetchStart = time.Unix(0, 0)
			}
			fetchEnd := end.Add(dur)

			klines, err := market.GetKlinesRange(symbol, tf, fetchStart, fetchEnd)
			if err != nil {
				return fmt.Errorf("fetch klines for %s %s: %w", symbol, tf, err)
			}
			if len(klines) == 0 {
				return fmt.Errorf("no klines for %s %s", symbol, tf)
			}

			series := &timeframeSeries{
				klines:     klines,
				closeTimes: make([]int64, len(klines)),
			}
			for i, k := range klines {
				series.closeTimes[i] = k.CloseTime
			}
			ss.byTF[tf] = series
		}
		df.symbolSeries[symbol] = ss
	}

	// Generate backtest progress timeline
	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	intervalMin := df.cfg.DecisionIntervalMinutes
	if intervalMin > 0 {
		// 与实盘一致：按固定分钟间隔推进（如 5 分钟 = 实盘 ScanIntervalMinutes）
		stepMs := int64(intervalMin) * 60 * 1000
		for t := startMs; t <= endMs; t += stepMs {
			df.decisionTimes = append(df.decisionTimes, t)
		}
	} else {
		// 按 K 线节奏：使用主周期 bar 的收盘时间
		firstSymbol := df.symbols[0]
		primarySeries := df.symbolSeries[firstSymbol].byTF[df.primaryTF]
		for _, ts := range primarySeries.closeTimes {
			if ts < startMs {
				continue
			}
			if ts > endMs {
				break
			}
			df.decisionTimes = append(df.decisionTimes, ts)
		}
	}
	for _, symbol := range df.symbols {
		if _, ok := df.symbolSeries[symbol].byTF[df.primaryTF]; !ok {
			return fmt.Errorf("symbol %s missing timeframe %s", symbol, df.primaryTF)
		}
	}
	if len(df.decisionTimes) == 0 {
		return fmt.Errorf("no decision bars in range")
	}
	return nil
}

func (df *DataFeed) DecisionBarCount() int {
	return len(df.decisionTimes)
}

func (df *DataFeed) DecisionTimestamp(index int) int64 {
	// Bounds check to prevent panic
	if index < 0 || index >= len(df.decisionTimes) {
		return 0
	}
	return df.decisionTimes[index]
}

func (df *DataFeed) sliceUpTo(symbol, tf string, ts int64) []market.Kline {
	// Nil checks to prevent panic
	ss, ok := df.symbolSeries[symbol]
	if !ok || ss == nil {
		return nil
	}
	series, ok := ss.byTF[tf]
	if !ok || series == nil {
		return nil
	}
	idx := sort.Search(len(series.closeTimes), func(i int) bool {
		return series.closeTimes[i] > ts
	})
	if idx <= 0 {
		return nil
	}
	return series.klines[:idx]
}

func (df *DataFeed) BuildMarketData(ts int64) (map[string]*market.Data, map[string]map[string]*market.Data, error) {
	result := make(map[string]*market.Data, len(df.symbols))
	multi := make(map[string]map[string]*market.Data, len(df.symbols))

	for _, symbol := range df.symbols {
		perTF := make(map[string]*market.Data, len(df.timeframes))
		for _, tf := range df.timeframes {
			series := df.sliceUpTo(symbol, tf, ts)
			if len(series) == 0 {
				continue
			}
			var longer []market.Kline
			if df.longerTF != "" && df.longerTF != tf {
				longer = df.sliceUpTo(symbol, df.longerTF, ts)
			}
			data, err := market.BuildDataFromKlines(symbol, series, longer)
			if err != nil {
				return nil, nil, err
			}
			perTF[tf] = data
			if tf == df.primaryTF {
				result[symbol] = data
			}
		}
		// Skip symbols without primary data instead of failing the entire backtest
		if _, ok := perTF[df.primaryTF]; !ok {
			// Log warning but continue with other symbols
			continue
		}
		multi[symbol] = perTF
	}
	return result, multi, nil
}

func (df *DataFeed) decisionBarSnapshot(symbol string, ts int64) (*market.Kline, *market.Kline) {
	ss, ok := df.symbolSeries[symbol]
	if !ok {
		return nil, nil
	}
	series, ok := ss.byTF[df.primaryTF]
	if !ok {
		return nil, nil
	}
	idx := sort.Search(len(series.closeTimes), func(i int) bool {
		return series.closeTimes[i] >= ts
	})
	if idx >= len(series.closeTimes) || series.closeTimes[idx] != ts {
		return nil, nil
	}
	curr := &series.klines[idx]
	var next *market.Kline
	if idx+1 < len(series.klines) {
		next = &series.klines[idx+1]
	}
	return curr, next
}

// GetBarAt returns the OHLC bar for the given symbol at the decision timestamp ts (bar that closes at ts).
// Used by backtest to check dynamic stop-loss/take-profit with bar high/low.
func (df *DataFeed) GetBarAt(symbol string, ts int64) *market.Kline {
	curr, _ := df.decisionBarSnapshot(symbol, ts)
	if curr == nil {
		return nil
	}
	k := *curr
	return &k
}

// KlinesUpTo returns klines for symbol up to (and including) ts, primary timeframe. For support/resistance.
func (df *DataFeed) KlinesUpTo(symbol string, ts int64) []market.Kline {
	slice := df.sliceUpTo(symbol, df.primaryTF, ts)
	if len(slice) == 0 {
		return nil
	}
	return slice
}
