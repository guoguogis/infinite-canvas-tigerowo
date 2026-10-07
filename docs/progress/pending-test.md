---
title: 待测试
description: 当前版本已实现但仍需人工验证的变更项
---

# 待测试

## 画布侧栏节点列表的媒体缩略图

- 左侧「画布元素」列表里，视频节点改为显示真实缩略图（`<video preload="metadata">` 取首帧），不再只显示视频图标。
- 验证点：视频节点有内容时列表中显示画面；节点无内容（空视频节点）时仍回退为图标；图片/全景图节点缩略图行为不变。

## 管理后台提示词视频预览

- `/admin/prompts` 的封面列与详情弹窗改为复用 `PromptCover` / `PromptPreview`，视频提示词可以直接播放（此前用的是 antd `Image`，只渲染封面图）。
- 封面列点击行为统一为打开详情弹窗；有 `videoUrl` 时封面显示播放角标。
- 新增/编辑弹窗补上「视频 URL」字段，便于手工维护视频地址。
- 验证点：视频提示词在表格封面显示播放角标，点开后弹窗内可播放；无视频的提示词行为与之前一致。

## 客户视图提示词只显示封面

- 客户视图（`/prompts` 页面的 `PromptCard` / `PromptDetailDialog`，画布侧栏的提示词详情复用同一个弹窗）不再播放示例视频，封面也不显示播放标识，只展示封面图。
- 视频预览能力只保留在管理后台 `/admin/prompts`。
- 验证点：`/prompts` 卡片与详情弹窗只有封面图、无播放角标；管理后台仍可点封面预览视频。

## 无封面提示词改用渐变封面

- 背景：库内 5757 条提示词中有 1615 条没有真实封面（seedance-2-prompts 1109 条、lanshu-video-prompts 494 条、xianyu 9 条、davidwu 3 条），两个上游数据源都不提供图片或视频字段，无法补齐真实封面。
- `PromptCover` 在封面缺失或加载失败时，改为渲染按标题哈希取色的渐变块（径向高光 + 135° 线性渐变），不再显示灰底文档图标。
- 同一提示词的色相稳定不变；视频提示词保留播放角标，不重复叠图标。
- 验证点：提示词库中原本空白的卡片显示彩色渐变块；同一条提示词刷新后颜色一致；有真实封面的卡片不受影响。

## 修复 Meta 渠道 MiniMax-H3 视频生成参数丢失

- 现象：在 `/video` 选择 Meta 渠道的 MiniMax-H3 生成视频，报上游错误 `content 不能为空 (2013)`。
- 原因：走账号代理时前端已经按协议拼好了 `content[]`（含首尾帧与参考素材角色），后端适配器却按项目内部格式从 `prompt`/`images` 重新拼装，导致 `content` 被整段丢弃。
- 修复：`buildMiniMaxVideoBody` 在 `content` 已存在时直接沿用，仅在缺失时按项目格式拼装；同时 `miniMaxVideoRatio` 不再过滤首尾帧模式必需的 `ratio=adaptive`。
- 回归用例：`handler/model_protocol_flow_test.go` 新增两条真实报文形态的用例（协议格式 `content[]`、含 `first_frame` 角色与 `adaptive`）。
- 验证点：`/video` 用 Meta 渠道 MiniMax-H3 提交文本生成视频不再报 `content 不能为空`；首尾帧模式下的 `ratio` 会按 `adaptive` 传给上游。

## 新增豆包语音合成（doubao-tts）协议

- 背景：`doubao-seed-tts-2.0` 挂在 `ark` 渠道下调用时会 404。实测 `POST /api/v3/audio/speech`、`/api/plan/v3/audio/speech`、`/tts`、`/audio/tts` 等路径全部不存在（用空 JSON 报文探路：`/chat/completions` 返回 400 表示路径存在，`/audio/speech` 返回 404 表示不存在），方舟模型 API 不提供语音合成接口；豆包语音合成是独立的语音服务，接口与鉴权都不同。
- 新增协议 `doubao-tts`，**TokenPlan 与后付费都支持，差异只体现在 baseUrl 上**：
  - TokenPlan：`baseUrl = https://openspeech.bytedance.com/api/v3/plan`
  - 后付费：`baseUrl = https://openspeech.bytedance.com/api/v3`
  - 协议内路径统一为 `/tts/unidirectional`，所以协议代码不需要按计费模式分支；两档要用各自的 key。
- 请求转换：`/audio/speech` → `/tts/unidirectional`，body 转成 `{req_params:{text,speaker,audio_params:{format,sample_rate}}}`，并补 `X-Api-Key`（渠道密钥）、`X-Api-Resource-Id` 与 `X-Control-Require-Usage-Tokens-Return`。
- 响应处理：上游是逐行 JSON 流，`code=0` 的 `data` 为 base64 音频分片，`code=20000000` 为结束标记；后端聚合成完整音频后按 `audio/*` 返回，画布音频任务与前端都无需改动。
- 模型差异表在 `service/doubao_tts.go`：`doubao-seed-tts-2.0` / `seed-tts-2.0` → `X-Api-Resource-Id: seed-tts-2.0` / 默认音色 `zh_female_vv_uranus_bigtts`；请求里的 `voice` 若本身是豆包音色（含 `_bigtts`）则优先使用。
- 实测（空 JSON 报文探路，不产生合成费用）：方舟 Agent Plan 的 `ark-` key 在 `.../api/v3/plan/tts/unidirectional` 返回 200，在 `.../api/v3/tts/unidirectional` 返回 401，说明该 key 属于 TokenPlan，后付费要另配 key。
- 未接入：`wss://.../tts/unidirectional/stream` 与 `wss://.../tts/bidirection` 是流式与双向实时接口，需要 WebSocket 客户端和另一套响应管线；当前只用 HTTP 单向接口，因为画布音频任务需要的是完整音频文件。
- 验证点：渠道协议下拉出现「豆包语音合成」；TokenPlan 渠道用 `ark-` key、后付费渠道用后付费 key，都能在画布音频节点生成语音；上游返回错误码时能透出真实错误信息。
- 音色：`web/src/lib/doubao-tts.ts` 内置 90 条 `*_uranus_bigtts` 音色（豆包语音合成 2.0，含中文名），按协议识别后音频设置与全局配置的音色下拉自动切换为可搜索列表；默认音色 `zh_female_shaoergushi_uranus_bigtts`（少儿故事），前后端兜底值一致。

## 音频设置支持按厂商选择

- 画布音频设置顶部新增「厂商」下拉，选项从当前已配置的音频模型里按厂商分组推导（豆包语音合成 / Gemini / GLM / MiMo / Grok / AutoDL / OpenAI 兼容），没有配置对应模型时不出现该厂商。
- 切换厂商会同时把节点的 `model` 与 `channelId` 切到该厂商的音频模型，音色下拉随即切换到该厂商的音色目录，避免"选了厂商但音色静默失效"。
- 厂商归属判定与音色目录共用同一套协议识别（`web/src/lib/audio-vendor.ts`），顺序即优先级，OpenAI 兼容放最后兜底。
- 验证点：画布音频节点设置里出现「厂商」；切到豆包后音色列表为 90 条豆包音色且默认选中「少儿故事」；切到其他厂商后音色列表随之变化，生成时按所选厂商的模型与渠道发起请求。

## 音频设置全部改为下拉

- 画布音频设置里的「声音」「格式」「语速」原来都是药丸按钮网格（`OptionPill`），现统一改为下拉；`OptionPill` 已移除。
- 声音与语速用可输入的下拉（`Select mode="tags" maxCount={1}`）：候选项显示中文名/中文标签，也可以直接输入自定义值 —— 音色可以直接粘贴音色 ID，语速可以直接填任意数值（提交时按各厂商范围归一）。
- 下拉过滤同时匹配候选值的 label 与 value，因此既能按中文名搜，也能按音色 ID 搜。
- 验证点：音频设置里不再出现按钮网格；音色下拉默认显示「少儿故事」；手动输入一个不在列表里的音色 ID 能保留并被发送；语速选预设或直接输入数值都生效。

## 公开配置新增默认音频配置（渠道-模型-音色 三级联动）

- 公开配置此前只有 `defaultModel` / `defaultImageModel` / `defaultVideoModel` / `defaultTextModel`，唯独没有音频默认值；现新增 `defaultAudioChannelId` / `defaultAudioModel` / `defaultAudioVoice`（`model/setting.go`）。
- 管理后台「公开配置」新增「默认音频配置」区：渠道 → 模型 → 音色 三级联动。模型选项按「厂商-模型名」展示（厂商取自协议识别，如 `豆包语音合成-doubao-seed-tts-2.0`）；音色可下拉选择，也可直接输入私有音色 ID（声音复刻）。
- 上游资源 ID 改为按**音色 ID** 推导（此前固定 `seed-tts-2.0`）：`S_` 开头与 `ICL_uranus_*` → `seed-icl-2.0`；含 `uranus` → `seed-tts-2.0`；其他 `_bigtts`（1.0 代）→ `volc.service_type.10029`。传错的表现是 HTTP 200 + `code 55000000` 空音频。
- 用户侧：`useEffectiveConfig` 优先使用公开配置的默认音频模型，并在用户未自选渠道/音色时套用公开配置的默认渠道与音色。
- 验证点：管理页能选出「渠道-模型-音色」，保存后重新打开回显一致；手填的私有音色 ID 能保存并在生成时带上正确的 `X-Api-Resource-Id`；未自行设置音频的用户会继承公开配置的默认音频模型。


- 未纳入的 `ICL_uranus_*` 角色音色需要 `X-Api-Resource-Id: seed-icl-2.0`，与当前固定 `seed-tts-2.0` 不一致，因此暂不放出；如需支持要按音色前缀扩展资源 ID。

## 全站强制登录与控制台首页

- `web/src/app/(user)/layout.tsx` 的守卫改为：除 `/login`、`/register`、`/tokendance/callback` 外，所有页面都要求登录；未登录时守卫直接渲染 `null` 并 `router.replace("/login?redirect=<原路径>")`，登录成功后跳回原路径。
- 访客（未登录）模式不再可达：未登录的画布/素材本地保存、未登录本地直连渠道、未登录本地 Skill 等场景代码仍在，但用户已无法进入。
- 首页改为「控制台」：按「我的画布 / 我的视频 / 我的图片」三行展示历史记录，原落地页已删除；顶栏品牌文字与浏览器标题由「无限画布」改为「控制台」；右上角 GitHub 图标入口与顶部广告栏已移除，`github-link.tsx` 与 `announcement-banner.tsx` 两个组件文件已删除；登录页不显示顶部菜单栏。
- 管理后台 `(admin)` 目录原本就有独立守卫，本次未改动。
- 验证点：未登录访问任意页面（画布、素材、提示词库、生图/视频工作台等）都会跳到登录页，登录后回到原路径；首页显示三行历史记录且「查看全部」可进入对应页面；顶栏不再出现 GitHub 图标和顶部广告栏；管理后台登录守卫行为不变。
