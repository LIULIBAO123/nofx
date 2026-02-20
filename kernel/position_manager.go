package kernel

import (
	"fmt"
	"nofx/logger"
	"time"
)

// PositionManager 仓位管理器
type PositionManager struct {
	config *PositionManagementConfig
}

// PositionManagementConfig 仓位管理配置
type PositionManagementConfig struct {
	// 金字塔加仓
	EnablePyramiding bool    `json:"enable_pyramiding"`
	MaxPyramidLevels int     `json:"max_pyramid_levels"` // 最多加仓次数
	PyramidSizeRatio float64 `json:"pyramid_size_ratio"` // 每次加仓比例递减
	MinProfitToAdd   float64 `json:"min_profit_to_add"`  // 最小盈利才能加仓

	// 分批止盈
	EnableScaledExit bool              `json:"enable_scaled_exit"`
	ScaledExitLevels []ScaledExitLevel `json:"scaled_exit_levels"`

	// 持仓时间管理
	MinHoldTime time.Duration `json:"min_hold_time"` // 最小持仓时间
	MaxHoldTime time.Duration `json:"max_hold_time"` // 最大持仓时间
}

// ScaledExitLevel 分批止盈级别
type ScaledExitLevel struct {
	ProfitThreshold float64 `json:"profit_threshold"` // 盈利阈值 (%)
	ExitPercent     float64 `json:"exit_percent"`     // 平仓比例 (%)
}

// NewPositionManager 创建仓位管理器
func NewPositionManager(config *PositionManagementConfig) *PositionManager {
	if config == nil {
		// 默认配置
		config = &PositionManagementConfig{
			EnablePyramiding: true,
			MaxPyramidLevels: 2,
			PyramidSizeRatio: 0.5,
			MinProfitToAdd:   1.0,
			EnableScaledExit: true,
			ScaledExitLevels: []ScaledExitLevel{
				{ProfitThreshold: 3.0, ExitPercent: 33},
				{ProfitThreshold: 5.0, ExitPercent: 50},
				{ProfitThreshold: 8.0, ExitPercent: 100},
			},
			MinHoldTime: 30 * time.Minute,
			MaxHoldTime: 4 * time.Hour,
		}
	}
	return &PositionManager{config: config}
}

// AddPositionSignal 加仓信号
type AddPositionSignal struct {
	Allowed    bool
	Reason     string
	SizeRatio  float64 // 相对于初始仓位的比例
	Confidence int
}

// CheckAddPosition 检查是否可以加仓
func (pm *PositionManager) CheckAddPosition(position *PositionInfo, currentAddCount int) *AddPositionSignal {
	if !pm.config.EnablePyramiding {
		return &AddPositionSignal{
			Allowed: false,
			Reason:  "金字塔加仓未启用",
		}
	}

	// 检查加仓次数
	if currentAddCount >= pm.config.MaxPyramidLevels {
		return &AddPositionSignal{
			Allowed: false,
			Reason:  fmt.Sprintf("已达到最大加仓次数 (%d/%d)", currentAddCount, pm.config.MaxPyramidLevels),
		}
	}

	// 检查是否盈利
	if position.UnrealizedPnLPct < pm.config.MinProfitToAdd {
		return &AddPositionSignal{
			Allowed: false,
			Reason:  fmt.Sprintf("盈利不足，需要>%.1f%%才能加仓 (当前%.2f%%)", pm.config.MinProfitToAdd, position.UnrealizedPnLPct),
		}
	}

	// 计算加仓比例（递减）
	sizeRatio := pm.config.PyramidSizeRatio
	for i := 1; i < currentAddCount; i++ {
		sizeRatio *= pm.config.PyramidSizeRatio
	}

	return &AddPositionSignal{
		Allowed:    true,
		Reason:     fmt.Sprintf("可以加仓：盈利%.2f%% > %.1f%%，第%d次加仓", position.UnrealizedPnLPct, pm.config.MinProfitToAdd, currentAddCount+1),
		SizeRatio:  sizeRatio,
		Confidence: 75, // 加仓信心度略低于初始开仓
	}
}

// PartialExitSignal 部分平仓信号
type PartialExitSignal struct {
	Triggered   bool
	Reason      string
	ExitPercent float64 // 平仓比例 (%)
	Level       int     // 触发的级别
}

// CheckPartialExit 检查是否应该部分平仓
func (pm *PositionManager) CheckPartialExit(position *PositionInfo, takenLevels map[int]bool) *PartialExitSignal {
	if !pm.config.EnableScaledExit {
		return &PartialExitSignal{Triggered: false}
	}

	// 检查每个级别
	for i, level := range pm.config.ScaledExitLevels {
		// 跳过已触发的级别
		if takenLevels[i] {
			continue
		}

		// 检查是否达到盈利阈值
		if position.UnrealizedPnLPct >= level.ProfitThreshold {
			return &PartialExitSignal{
				Triggered:   true,
				Reason:      fmt.Sprintf("分批止盈: 盈利%.2f%% >= %.1f%%，平仓%.0f%%", position.UnrealizedPnLPct, level.ProfitThreshold, level.ExitPercent),
				ExitPercent: level.ExitPercent,
				Level:       i,
			}
		}
	}

	return &PartialExitSignal{Triggered: false}
}

// HoldTimeSignal 持仓时间信号
type HoldTimeSignal struct {
	Action string // "hold", "consider_exit", "force_exit"
	Reason string
}

// CheckHoldTime 检查持仓时间
func (pm *PositionManager) CheckHoldTime(position *PositionInfo) *HoldTimeSignal {
	if position.UpdateTime == 0 {
		return &HoldTimeSignal{Action: "hold", Reason: "持仓时间未知"}
	}

	holdDuration := time.Since(time.UnixMilli(position.UpdateTime))

	// 检查最小持仓时间
	if holdDuration < pm.config.MinHoldTime {
		return &HoldTimeSignal{
			Action: "hold",
			Reason: fmt.Sprintf("持仓时间过短 (%.0f分钟 < %.0f分钟)，避免频繁交易", holdDuration.Minutes(), pm.config.MinHoldTime.Minutes()),
		}
	}

	// 检查最大持仓时间
	if holdDuration > pm.config.MaxHoldTime {
		// 如果盈利不足，建议平仓
		if position.UnrealizedPnLPct < 1.0 {
			return &HoldTimeSignal{
				Action: "force_exit",
				Reason: fmt.Sprintf("持仓时间过长 (%.1f小时 > %.1f小时) 且盈利不足 (%.2f%%)，释放资金", holdDuration.Hours(), pm.config.MaxHoldTime.Hours(), position.UnrealizedPnLPct),
			}
		} else {
			return &HoldTimeSignal{
				Action: "consider_exit",
				Reason: fmt.Sprintf("持仓时间较长 (%.1f小时)，考虑止盈", holdDuration.Hours()),
			}
		}
	}

	return &HoldTimeSignal{
		Action: "hold",
		Reason: fmt.Sprintf("持仓时间正常 (%.0f分钟)", holdDuration.Minutes()),
	}
}

// LogPositionManagement 记录仓位管理决策
func LogPositionManagement(symbol string, signal interface{}) {
	switch s := signal.(type) {
	case *AddPositionSignal:
		if s.Allowed {
			logger.Infof("📈 加仓信号: %s - %s (比例: %.0f%%)", symbol, s.Reason, s.SizeRatio*100)
		}
	case *PartialExitSignal:
		if s.Triggered {
			logger.Infof("💰 分批止盈: %s - %s", symbol, s.Reason)
		}
	case *HoldTimeSignal:
		if s.Action != "hold" {
			logger.Infof("⏱️  持仓时间: %s - %s", symbol, s.Reason)
		}
	}
}

