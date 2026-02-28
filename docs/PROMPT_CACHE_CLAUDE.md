# Prompt 缓存显示为 0 的说明（Claude / 中转站）

## 现象

前端「Prompt 缓存」显示：**读取 0 · 创建 0**，且一直不变。

## 本端行为（已实现）

- 请求 **Anthropic 格式**：`POST /v1/messages`，请求体包含：
  - `system`、`messages`、`max_tokens`、`cache_control: { "type": "ephemeral" }`
- 只有带上 `cache_control` 时，Anthropic 才会启用 prompt caching，并在响应 `usage` 中返回：
  - `cache_read_input_tokens`
  - `cache_creation_input_tokens`
- 响应解析已支持上述字段；并对中转站可能返回的 `prompt_tokens`/`completion_tokens` 等做了兼容。

## 使用 Claude 中转站时缓存为 0 的常见原因

结合你提供的中转站截图（anthropic `/v1/messages`、Default-cache 分组等），可能原因包括：

1. **请求未带 `cache_control` 或未被转发**
   - 中转若把请求体里的 `cache_control` 去掉或未原样转发给 Anthropic，Anthropic 不会启用缓存，返回的缓存 token 为 0。
   - **建议**：在中转站或抓包中确认请求体是否包含 `"cache_control": { "type": "ephemeral" }`。

2. **计费走的是 default 分组而非 Default-cache**
   - 截图里计费过程按「提示 24586 tokens」整笔计费，未区分缓存读取/创建，说明本次请求可能走的是 **default 分组**，未走 **Default-cache 分组**。
   - 若中转的「自动分组调用链路」没有把可缓存请求路由到 Default-cache，则计费与响应里都可能不体现缓存，前端就会一直看到 0。

3. **中转响应未返回缓存字段**
   - Anthropic 原样响应会在 `usage` 里带 `cache_read_input_tokens`、`cache_creation_input_tokens`。
   - 若中转在聚合/计费时重写或丢弃了这两个字段，本端解析到的就一直是 0。
   - **建议**：直接看中转返回的 JSON 里 `usage` 是否包含上述两个 key。

4. **首轮或 prompt 前缀经常变化**
   - 即使一切配置正确，**第一次请求**没有已有缓存，多为：读取 0，创建 = 本次写入缓存的 token 数。
   - 若每轮请求的 system/user 前缀变化较大，缓存命中率会低，读取也可能长期接近 0。
   - **回测场景**：每根 K 线/每个周期 market data、持仓、时间戳都不同，prompt 前缀几乎每次都变，所以回测时「创建」有值、「读取」一直为 0 是正常现象；实盘同策略、同 system prompt 且前缀稳定时，读取才会逐渐出现。

## 建议排查步骤

1. 确认请求：Body 中是否有 `cache_control: { type: "ephemeral" }`，且请求是发往 **anthropic `/v1/messages`**。
2. 确认响应：看原始 JSON 的 `usage` 是否包含 `cache_read_input_tokens`、`cache_creation_input_tokens`。
3. 确认分组：在中转站查看该次请求是否走 **Default-cache** 计费；若始终走 default，需在中转侧调整路由或配置，使 Claude 请求能走缓存分组。

结论：**本端已按 Anthropic 规范发送 `cache_control` 并解析缓存字段；若仍显示 0，问题通常在中转未转发/未使用缓存或未返回缓存统计。**

---

## 详细确认步骤（按顺序做）

### 一、确认请求体含 cache_control，且请求发往 anthropic /v1/messages

**目的**：确认 NOFX 发出的请求没有被中转改掉或发错接口。

| 步骤 | 操作 | 要看什么 |
|------|------|----------|
| 1.1 | 在中转站打开「请求日志」「调用记录」或「API 日志」等页面，找到最近一条 **Claude/该模型** 的请求（时间与你在 NOFX 里触发的一次 AI 决策对应）。 | 能定位到单次请求的详情。 |
| 1.2 | 点进该条请求，查看 **请求 URL / 路径**。 | 应为 **`/v1/messages`** 或 base 为 anthropic 的 `.../v1/messages`。若是 `/v1/chat/completions` 等 OpenAI 路径，则不是 Anthropic 原生格式，缓存可能不被支持。 |
| 1.3 | 在同一详情里查看 **请求体 (Request Body / Body)**，打开原始 JSON。 | 在顶层应能看到：`"cache_control": { "type": "ephemeral" }`（或 `"type":"ephemeral"`）。若完全没有 `cache_control`，或 type 不是 `ephemeral`，说明请求未带缓存参数或被中转去掉了。 |
| 1.4（可选） | 在 NOFX 所在机器用抓包工具（如 Fiddler、Charles、或浏览器 F12 看发往中转站的请求）。 | 若 NOFX 配置的 API 地址是中转站，则发往中转的请求体里应已包含 `cache_control`；若这里没有，问题在本端；若有而中转日志里没有，说明中转在入站时就去掉了。 |

**通过标准**：请求 URL 为 `/v1/messages`，且 Body 中有 `"cache_control": { "type": "ephemeral" }`。

---

### 二、确认该次请求是否走 Default-cache 分组计费

**目的**：确认中转站是否把这次调用算在「缓存分组」上；若一直走 default，计费与统计往往不会体现缓存。

| 步骤 | 操作 | 要看什么 |
|------|------|----------|
| 2.1 | 在同一条请求的详情里，查看「计费分组」「扣费分组」「分组」或「Billing Group」等字段。 | 应显示 **Default-cache**（或你为缓存单独建的分组名）。若显示 **default**、**vip** 等非缓存分组，说明本次未走缓存分组。 |
| 2.2 | 查看该条请求的「计费过程」「费用明细」或「日志详情」。 | 若走缓存，通常会看到与「缓存」相关的倍率或拆分，例如「缓存倍率」「缓存读取」「缓存创建」等。若只看到「提示 xxx tokens」「补全 xxx tokens」按统一单价计费、没有任何缓存相关行，多半是走 default 分组。 |
| 2.3 | 在中转站「模型配置」或「分组价格」里，看该模型的「自动分组调用链路」（或等价配置）。 | 确认链路里是否包含 **Default-cache**，以及触发条件（例如：带 cache_control 的请求是否会被路由到 Default-cache）。若链路只有 default → vip，没有 Default-cache，则请求永远不会走缓存分组。 |

**通过标准**：该次请求的分组为 Default-cache，且计费明细里能看到与缓存相关的项（或至少分组名是 Default-cache）。

---

### 三、确认原始响应的 usage 里是否有缓存相关字段

**目的**：确认 Anthropic 返回的缓存统计有没有被中转原样透传；若响应里就没有，前端必然显示 0。

| 步骤 | 操作 | 要看什么 |
|------|------|----------|
| 3.1 | 在同一条请求的详情里，找到 **响应体 (Response Body / Response)**，打开原始 JSON。 | 需要看到完整的 HTTP 响应 body，而不是「已解析」的摘要。 |
| 3.2 | 在响应 JSON 里找到 **`usage`** 对象。 | 一般格式类似：`"usage": { "input_tokens": 24586, "output_tokens": 833, ... }`。 |
| 3.3 | 在 `usage` 里查找这两个 key：`cache_read_input_tokens`、`cache_creation_input_tokens`。 | **有**：说明 Anthropic 返回了缓存统计，且中转原样透传了；若此时前端仍为 0，可能是前端或 NOFX 解析问题（可把该段 usage 贴给开发排查）。**没有**：说明要么 Anthropic 没返回（例如请求没带 cache_control），要么中转在返回前删掉了这两个字段，前端只能显示 0。 |
| 3.4（可选） | 用同一账号/模型，直接调 Anthropic 官方 API（带 cache_control），看官方返回的 usage。 | 对比官方响应里是否有 `cache_read_input_tokens`、`cache_creation_input_tokens`；若官方有而经中转的没有，可确定是中转改写或丢弃了字段。 |

**通过标准**：响应 JSON 中 `usage` 对象内存在 `cache_read_input_tokens` 和 `cache_creation_input_tokens`（值可以为 0，但 key 要存在）。

---

### 四、结果对照表

| 检查项 | 通过 | 未通过时可能原因 |
|--------|------|------------------|
| 请求路径为 /v1/messages，Body 含 cache_control | 是 | 请求被转到 OpenAI 格式接口；或中转/网关去掉了 cache_control。 |
| 计费走 Default-cache 分组 | 是 | 分组链路未配置 Default-cache，或路由条件不满足。 |
| 响应 usage 含 cache_read/cache_creation | 是 | Anthropic 未返回（如未带 cache_control）；或中转重写/删除了 usage 字段。 |

三样都通过后，前端应能显示非零的缓存统计（至少从第二次相同前缀请求开始会有读取或创建）。若仍为 0，可把该条请求的「请求体」和「响应体」中的 usage 部分脱敏后提供给技术支持进一步排查。
