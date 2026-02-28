# Prompt Caching 使用与验证

nofx 已启用 **Prompt Caching**（系统提示词缓存），在调用支持缓存的模型（如 Claude、OpenAI）时会自动对 system 消息启用 `cache_control: ephemeral`，以降低重复系统提示的计费与延迟。

## 如何确认在正常运行

### 1. 前端页面查看（推荐）

- **策略工作室**：打开「策略工作室」→ 右侧切换到 **「测试」** 标签，在「运行测试」按钮下方会显示 **「Token 用量 / Prompt 缓存」** 卡片。完成一次 AI 测试或回测后，这里会显示最近一次调用的 token 数；若模型支持并启用了缓存，会显示 **Prompt 缓存: 读取 X · 创建 Y**。
- **回测实验室**：进入某次回测的详情（选中一条运行），在「当前净值 / 总收益率 / 最大回撤 / 夏普」统计卡片下方会显示同一块 **AI 用量** 卡片，回测过程中有 AI 决策后即可看到用量与缓存统计。

卡片每约 10 秒自动刷新；若尚未有过 AI 调用，会显示「暂无数据（完成一次 AI 调用后显示）」。

### 2. 看后端日志

当 AI 返回的 usage 里包含缓存相关字段时，会在**后端日志**中打印一行，例如：

```
📦 Prompt Caching: read=12345 created=0 (provider=anthropic model=claude-sonnet-4-20250514)
```

- **read**：从缓存读取的 input token 数（>0 表示命中缓存）
- **created**：本次请求用于创建/写入缓存的 input token 数（首次或缓存失效时 >0）

**怎么看：**

- **Docker**：`docker compose logs -f nofx`，跑一次回测或实盘决策后看是否出现上述 `📦 Prompt Caching` 行
- **本地**：直接看运行 nofx 的终端输出

只要在调用 Claude/OpenAI 后偶尔或持续看到 `read > 0` 或 `created > 0`，就说明 Prompt Caching 在正常使用。

### 3. 代码层面如何启用

- **策略/回测**：`kernel.callAIWithCaching()` 会使用带 `cache_control` 的 Request API 发起调用；若 Request API 不可用会回退到普通 `CallWithMessages`（无缓存统计）。
- **MCP 请求**：system 消息通过 `mcp.NewSystemMessageWithCache(systemPrompt)` 构造，会在请求体中带上 `cache_control: { type: "ephemeral" }`。

### 4. 为什么前端一直显示「Prompt 缓存: 读取 0 · 创建 0」？

可能原因与处理：

| 原因 | 说明 |
|------|------|
| **请求未带 cache_control（已修复）** | 之前只有通过 `CallWithRequest` 的请求会带 `cache_control`；策略测试、回测交易分析等走 `CallWithMessages` 时 Claude 未带该参数，API 不返回缓存字段，故一直为 0。现已改为 **Claude 的 `CallWithMessages` 也统一带上 `cache_control: { type: "ephemeral" }`**，所有 Claude 调用都应能拿到缓存统计。 |
| **模型/地区不支持** | Anthropic 仅部分模型/区域支持 Prompt Caching 并返回 `cache_read_input_tokens` / `cache_creation_input_tokens`，若你用的端点或模型不支持，会始终为 0。可查 [Anthropic 文档](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching) 确认当前模型与区域。 |
| **DeepSeek** | DeepSeek 返回 `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens`，已映射为「读取 / 创建」。首次请求多为「创建」>0，后续相同前缀请求会出现「读取」>0；若每次 prompt 差异很大，可能长期以「创建」为主。 |
| **仅首次或单次调用** | 第一次调用会「创建」缓存（创建 >0、读取 0）；同一会话内再次调用且 system/前缀一致时才会出现「读取」>0。多跑几轮决策或测试后再看。 |

建议：确认使用已支持缓存的模型（如 Claude、DeepSeek），完成一次部署更新后多发起几次 AI 调用（回测或策略测试），再查看 Token 用量卡片或日志中的 `📦 Prompt Caching`。

## 参考

- [Anthropic – Prompt caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching)
- 本仓库：`kernel/engine.go`（callAIWithCaching）、`mcp/request.go`（CacheControl）、`mcp/claude_client.go`（usage 解析）
