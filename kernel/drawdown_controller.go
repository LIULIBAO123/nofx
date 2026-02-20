package kernel

import (
	"fmt"
	"nofx/logger"
	"time"
)

// DrawdownController 回撤控制器
type DrawdownController struct {
	config          *DrawdownControlConfig
	initialEquity   float64
	peakEquity      float64
	currentDrawdown float64
	recoveryMode    bool
	lastUpdateTime  time.Time
}

// DrawdownControlConfig 回撤控制配置
type DrawdownControlConfig struct {
	MaxDrawdownLimit float64 `json:"max_drawdown_limit"` // 最大回撤限制 (0.20 = 20%)

	// 回撤分级响应
	DrawdownLevels []DrawdownLevel `json:"drawdown_levels"`

	// 恢复机制
	RecoveryThreshold    float64 `json:"recovery_threshold"`     // 恢复阈值 (回撤降到此值以下退出恢复模式)
	RecoveryPositionSize float64 `json:"recovery_position_size"` // 恢复期间的仓位限制 (0.5 = 50%)
}

// DrawdownLevel 回撤级别
type DrawdownLevel struct {
	Threshold      float64 `json:"threshold"`       // 回撤阈值 (0.10 = 10%)
	Action         string  `json:"action"`          // "reduce_size", "stop_new_trades", "close_all"
	PositionSizeMultiplier float64 `json:"position_size_multiplier,omitempty"` // 仓位缩减倍数
}

// NewDrawdownController 创建回撤控制器
func NewDrawdownController(config *DrawdownControlConfig, initialEquity float64) *DrawdownController {
	if config == nil {
		// 默认配置
		config = &DrawdownControlConfig{
			MaxDrawdownLimit: 0.20, // 20%
			DrawdownLevels: []DrawdownLevel{
				{Threshold: 0.10, Action: "reduce_size", PositionSizeMultiplier: 0.5},      // 10%回撤减仓50%
				{Threshold: 0.15, Action: "stop_new_trades"},                                // 15%回撤停止新开仓
				{Threshold: 0.20, Action: "close_all"},                                      // 20%回撤全部平仓
			},
			RecoveryThreshold:    0.05, // 回撤降到5%以下退出恢复模式
			RecoveryPositionSize: 0.5,  // 恢复期间仓位限制50%
		}
	}

	return &DrawdownController{
		config:         config,
		initialEquity:  initialEquity,
		peakEquity:     initialEquity,
		currentDrawdown: 0,
		recoveryMode:   false,
		lastUpdateTime: time.Now(),
	}
}

// Update 更新回撤状态
func (dc *DrawdownController) Update(currentEquity float64) {
	// 更新峰值权益
	if currentEquity > dc.peakEquity {
		dc.peakEquity = currentEquity
	}

	// 计算当前回撤
	if dc.peakEquity > 0 {
		dc.currentDrawdown = (dc.peakEquity - currentEquity) / dc.peakEquity
	}

	// 检查是否应该退出恢复模式
	if dc.recoveryMode && dc.currentDrawdown < dc.config.RecoveryThreshold {
		dc.recoveryMode = false
		logger.Infof("✅ 退出恢复模式: 回撤降至%.2f%% < %.2f%%", dc.currentDrawdown*100, dc.config.RecoveryThreshold*100)
	}

	dc.lastUpdateTime = time.Now()
}

// DrawdownSignal 回撤信号
type DrawdownSignal struct {
	CurrentDrawdown float64 // 当前回撤
	PeakEquity      float64 // 峰值权益
	Action          string  // "normal", "reduce_size", "stop_new_trades", "close_all"
	Reason          string
	PositionSizeMultiplier float64 // 仓位缩减倍数
	RecoveryMode    bool
}

// CheckDrawdown 检查回撤状态
func (dc *DrawdownController) CheckDrawdown() *DrawdownSignal {
	signal := &DrawdownSignal{
		CurrentDrawdown:        dc.currentDrawdown,
		PeakEquity:             dc.peakEquity,
		Action:                 "normal",
		PositionSizeMultiplier: 1.0,
		RecoveryMode:           dc.recoveryMode,
	}

	// 检查每个回撤级别（从高到低）
	for i := len(dc.config.DrawdownLevels) - 1; i >= 0; i-- {
		level := dc.config.DrawdownLevels[i]
		if dc.currentDrawdown >= level.Threshold {
			signal.Action = level.Action
			signal.Reason = fmt.Sprintf("回撤达到%.1f%% (阈值%.1f%%)，触发: %s", 
				dc.currentDrawdown*100, level.Threshold*100, getActionDescription(level.Action))
			
			if level.Action == "reduce_size" && level.PositionSizeMultiplier > 0 {
				signal.PositionSizeMultiplier = level.PositionSizeMultiplier
			}

			// 进入恢复模式
			if !dc.recoveryMode {
				dc.recoveryMode = true
				logger.Warnf("⚠️  进入恢复模式: 回撤%.2f%% >= %.2f%%", dc.currentDrawdown*100, level.Threshold*100)
			}

			return signal
		}
	}

	// 恢复模式下的限制
	if dc.recoveryMode {
		signal.Action = "recovery_mode"
		signal.Reason = fmt.Sprintf("恢复模式: 回撤%.1f%%，仓位限制%.0f%%", dc.currentDrawdown*100, dc.config.RecoveryPositionSize*100)
		signal.PositionSizeMultiplier = dc.config.RecoveryPositionSize
	}

	return signal
}

// GetCurrentDrawdown 获取当前回撤
func (dc *DrawdownController) GetCurrentDrawdown() float64 {
	return dc.currentDrawdown
}

// GetPeakEquity 获取峰值权益
func (dc *DrawdownController) GetPeakEquity() float64 {
	return dc.peakEquity
}

// IsRecoveryMode 是否处于恢复模式
func (dc *DrawdownController) IsRecoveryMode() bool {
	return dc.recoveryMode
}

// GetStatus 获取状态摘要
func (dc *DrawdownController) GetStatus() string {
	if dc.recoveryMode {
		return fmt.Sprintf("恢复模式 (回撤%.2f%%)", dc.currentDrawdown*100)
	}
	return fmt.Sprintf("正常 (回撤%.2f%%)", dc.currentDrawdown*100)
}

// getActionDescription 获取动作描述
func getActionDescription(action string) string {
	switch action {
	case "reduce_size":
		return "减少仓位"
	case "stop_new_trades":
		return "停止新开仓"
	case "close_all":
		return "全部平仓"
	case "recovery_mode":
		return "恢复模式"
	default:
		return "正常交易"
	}
}

// LogDrawdownStatus 记录回撤状态
func LogDrawdownStatus(signal *DrawdownSignal) {
	if signal.Action != "normal" {
		logger.Warnf("📉 回撤控制: %s", signal.Reason)
		if signal.PositionSizeMultiplier < 1.0 {
			logger.Warnf("   仓位限制: %.0f%%", signal.PositionSizeMultiplier*100)
		}
	}
}

// ShouldAllowNewTrade 是否允许新开仓
func (dc *DrawdownController) ShouldAllowNewTrade() (bool, string) {
	signal := dc.CheckDrawdown()
	
	switch signal.Action {
	case "stop_new_trades", "close_all":
		return false, signal.Reason
	case "recovery_mode":
		return true, signal.Reason // 恢复模式允许交易但限制仓位
	default:
		return true, ""
	}
}

// AdjustPositionSize 调整仓位大小
func (dc *DrawdownController) AdjustPositionSize(originalSize float64) float64 {
	signal := dc.CheckDrawdown()
	return originalSize * signal.PositionSizeMultiplier
}

// ShouldCloseAllPositions 是否应该全部平仓
func (dc *DrawdownController) ShouldCloseAllPositions() (bool, string) {
	signal := dc.CheckDrawdown()
	if signal.Action == "close_all" {
		return true, signal.Reason
	}
	return false, ""
}

