package coinglass

import (
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"nofx/logger"

	"github.com/gorilla/websocket"
)

const (
	// DefaultWSBaseURL KeyStore WSS 根路径（不含 query）
	DefaultWSBaseURL = "wss://www.keystore.com.cn/api/v1/ws/coinglass"
	// WSS 订阅频道（与文档一致）
	ChannelFundingRate  = "funding-rate"
	ChannelLiquidation   = "liquidation"
	ChannelOpenInterest = "open-interest"
	ChannelPrice        = "price"
	writeWait            = 10 * time.Second
	readWait             = 60 * time.Second
	pingPeriod           = 25 * time.Second
	pongWait             = 30 * time.Second
)

// WSClient Coinglass 经 KeyStore 的 WSS 客户端，订阅融资率、清算、未平仓、价格并缓存最新一条。
type WSClient struct {
	baseURL string
	apiKey  string
	mu      sync.RWMutex
	latest  map[string][]byte // channel -> 最新消息 JSON
	conn    *websocket.Conn
	closed  bool
}

// NewWSClient 创建 WSS 客户端。baseURL 为空时用 DefaultWSBaseURL；apiKey 为 KeyStore API Key。
func NewWSClient(baseURL, apiKey string) *WSClient {
	if baseURL == "" {
		baseURL = DefaultWSBaseURL
	}
	return &WSClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		latest:  make(map[string][]byte),
	}
}

// WSMessage 订阅/心跳用 JSON 结构
type WSMessage struct {
	Action   string   `json:"action,omitempty"`
	Channels []string `json:"channels,omitempty"`
	Type     string   `json:"type,omitempty"`
	Message  string   `json:"message,omitempty"`
}

// Connect 建立 WSS 连接并发送订阅。建议在 goroutine 中调用 Run 读循环。
func (w *WSClient) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	u, err := url.Parse(w.baseURL)
	if err != nil {
		return err
	}
	if w.apiKey != "" {
		q := u.Query()
		q.Set("api_key", w.apiKey)
		u.RawQuery = q.Encode()
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	w.conn = conn
	conn.SetReadLimit(1 << 20)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readWait))
		return nil
	})
	// 订阅四个频道
	sub := WSMessage{
		Action:   "subscribe",
		Channels: []string{ChannelFundingRate, ChannelLiquidation, ChannelOpenInterest, ChannelPrice},
	}
	raw, _ := json.Marshal(sub)
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		conn.Close()
		return err
	}
	logger.Infof("Coinglass WSS: connected and subscribed to funding-rate, liquidation, open-interest, price")
	return nil
}

// Run 在阻塞循环中读消息：应用层 ping 回复 pong，其余按 channel 缓存最新一条。调用者应在 goroutine 中执行。
func (w *WSClient) Run() {
	w.mu.RLock()
	conn := w.conn
	w.mu.RUnlock()
	if conn == nil {
		return
	}
	conn.SetReadDeadline(time.Now().Add(readWait))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			logger.Warnf("Coinglass WSS read: %v", err)
			return
		}
		var msg struct {
			Type    string `json:"type"`
			Channel string `json:"channel"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &msg)
		if msg.Type == "ping" {
			pong := WSMessage{Type: "pong", Message: msg.Message}
			raw, _ := json.Marshal(pong)
			w.mu.Lock()
			if w.conn != nil {
				_ = w.conn.WriteMessage(websocket.TextMessage, raw)
			}
			w.mu.Unlock()
			continue
		}
		if msg.Channel != "" {
			w.mu.Lock()
			w.latest[msg.Channel] = data
			w.mu.Unlock()
		}
		conn.SetReadDeadline(time.Now().Add(readWait))
	}
}

// GetLatest 返回某频道最新一条消息的 JSON；无则 ok=false。
func (w *WSClient) GetLatest(channel string) ([]byte, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	b, ok := w.latest[channel]
	if !ok || len(b) == 0 {
		return nil, false
	}
	return b, true
}

// Close 关闭连接
func (w *WSClient) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.conn != nil {
		_ = w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = w.conn.Close()
		w.conn = nil
	}
}
