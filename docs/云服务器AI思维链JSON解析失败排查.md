# 云服务器上「Model didn't output structured JSON decision」排查说明

## 现象

- **云服务器**：AI 思维链周期中频繁出现 `Model didn't output structured JSON decision, entering safe wait`，决策退化为 wait。
- **本地 Docker**：同一配置下很少或不会出现。

## 原因分析

后端在解析 AI 返回内容时，会依次尝试：

1. 从 `<decision>...</decision>` 中提取 JSON
2. 从 ` ```json ... ``` ` 代码块中提取
3. 用正则 `[\s*\{.*?\}\s*]` 在全文查找 JSON 数组

当**以上都匹配不到**时，就会进入 safe wait，并打出上述提示。云上与本地差异通常来自以下几类。

### 1. 响应被截断（最常见）

- 云上请求 AI 的 **prompt 可能更大**（更多持仓/更多候选币/更多 K 线），或模型先输出大段推理，**在输出到 `<decision>` 或 JSON 之前就用满了 max_tokens**，导致返回内容里根本没有完整 JSON。
- 本地网络快、或测试数据少，prompt 更短，不易触达上限。

**处理建议：**

- 在云服务器 `.env` 中**提高 `AI_MAX_TOKENS`**（默认 2000 偏小），例如：
  ```bash
  AI_MAX_TOKENS=8000
  ```
- 若使用 Claude/推理较长的模型，可再提高到 12000+，再重启后端。

### 2. 超时导致响应不完整

- 云服务器到 AI 接口（如 DeepSeek/OpenAI）的**网络延迟**通常比本机高，容易在「模型还没输出完」时就超时，拿到的 response 不完整，自然没有 JSON。

**处理建议：**

- 在 `.env` 中提高超时时间，例如：
  ```bash
  AI_TIMEOUT_SECONDS=120
  ```
  或 300（5 分钟），视模型与接口情况调整，然后重启后端。

### 3. 网络 / 代理 / 地域

- 云服务器在海外访问国内 API（或反之）时，可能遇到不稳定、重置连接、代理改写 body 等情况，导致：
  - 响应被截断
  - 或出现异常字符/编码，使正则匹配不到 JSON

**处理建议：**

- 尽量让云服务器与 AI 接口**同地域/同线路**（例如都在国内或都在同一云厂商）。
- 若走代理，确认代理不会改写响应 body；必要时在云上直接调 API 测试一次。

### 4. 编码 / 不可见字符

- 少数情况下，响应中带有不可见字符或全角括号等，现有 `fixMissingQuotes` 会替换部分，但若模型输出格式差异大，仍可能匹配不到。

**处理建议：**

- 查看后端日志中 `[SafeFallback]` 的 **jsonPart snippet / tail**，确认是否明显被截断或格式异常，再针对性调整策略或清洗逻辑。

## 如何确认是否是「截断」

1. 在云服务器上查看 nofx 后端日志（例如 `docker compose logs -f nofx`）。
2. 出现 safe wait 时，日志会打印：
   - `response len=...`
   - `jsonPart snippet` 或 `jsonPart tail(500)`
3. 若 **tail 末尾是半句话或半个 JSON**（例如只有 `[{"symbol"` 没有闭合的 `]`），基本可以判断是**响应被截断**，优先按上面提高 `AI_MAX_TOKENS` 和 `AI_TIMEOUT_SECONDS`。

## 推荐云服务器 .env 配置示例

```bash
# 提高输出长度上限，减少思维链被截断
AI_MAX_TOKENS=8000

# 提高超时，避免云到 API 延迟导致未收全响应
AI_TIMEOUT_SECONDS=120
```

修改后执行：

```bash
docker compose restart nofx
# 或 docker compose -f docker-compose.prod.yml restart nofx
```

若仍有大量 safe wait，可把日志里 `[SafeFallback]` 附近的几行贴出来，便于进一步判断是截断、超时还是格式问题。
