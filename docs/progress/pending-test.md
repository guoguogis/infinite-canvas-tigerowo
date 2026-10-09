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

## 浏览器本地数据按账号隔离

- `web/src/lib/user-localforage.ts` 新增 `userLocalStore(storeName)`：返回惰性代理，按当前登录用户解析独立的 localforage object store（命名 `<用户id>__<原名>`），未登录回退 `guest`。因为 zustand persist 在模块加载时就会读取一次本地数据，用户 id 会先从已持久化的登录 token（JWT 的 `userId` / `sub`）同步解析，保证首次加载就读到自己的命名空间。
- 已切换的库：`video_generation_logs`、`image_generation_logs`、`image_generation_categories`、`media_files`、`image_files`、`creative_workflows`、`agent_skills`、`app_state`（画布项目与素材 store）。`workflow_channels` 原本就以账号 id 作为 key，未改动。
- 新增 `useUserLocalStorageScope()`（挂在 `(user)/layout.tsx`）：登录用户确定后对画布与素材 store 执行 `persist.rehydrate()`，因此同一浏览器换号登录不会看到上一个账号的数据；本地画布为空且账号同步开启时会从账号补拉一次。
- 旧数据不迁移：改动前的全局命名空间数据不再被读取；画布项目会由账号同步补回，纯本地且未同步到账号的媒体不会恢复。
- 验证点：同一浏览器登录 A 账号建画布、生成视频，退出后登录 B 账号，B 看不到 A 的画布、视频、图片与素材；A 重新登录后数据仍在。

## 管理后台显示顶部菜单

- 管理后台是独立的 `(admin)` 路由组，不复用 `(user)/layout.tsx`，因此一直只有自己的 antd 侧边栏 + 标题栏，没有任何回到其它页面的入口（侧边栏里只有一个「前往画布」按钮）。现在在后台布局顶部加上 `AppTopNav`，控制台入口、站点导航、头像与任务入口都可直接使用。
- 同时去掉了后台标题栏右侧原本的 `UserStatusActions`（与顶部菜单完全重复，会同时出现两个头像）；后台页标题与侧边栏不变。
- 侧边栏底部的「前往画布」按钮一并去掉（顶部菜单已有「我的画布」，功能重复），现在底部只保留「退出登录」。
- 布局高度从固定 `100vh` 改为「顶部菜单 + `flex-1` 自适应」，侧边栏底部按钮与内容区滚动行为不变。
- 验证点：进入 `/admin/users` 顶部出现完整菜单且只有一个头像；侧边栏、页标题「用户管理」、表格正常；侧边栏底部只剩「退出登录」且仍在视口内；点顶部导航能离开后台进入其它页面。

## 画布详情页显示顶部菜单

- `AppTopNav` 原来用 `hideHeader = /^\/canvas\/[^/]+/.test(pathname)` 在画布详情页隐藏整条顶部菜单，现在去掉了这个判断，画布详情页与其它页面一样显示顶部菜单（控制台入口、导航、头像与任务入口）。
- 画布自身的悬浮工具栏保持不变：它承载着顶部菜单没有的内容（算力点余额、快捷键入口、Agent 展开按钮），因此没有一并移除，但配置/主题/头像等入口与顶部菜单存在重复。
- 验证点：进入 `/canvas/<id>` 后顶部出现控制台菜单与头像；画布本体正常渲染、画布菜单按钮与算力点余额仍在；画布区域高度自适应，不再被顶栏挤出屏幕。

## 音乐创作台（`/music`）

设计稿与原型在 `docs/design/music-studio/`（`prototype.html` 可交互原型、`compare.html` 竞品与能力对标）。

**后端**
- 新增 `music_tasks` 表与 `model/repository/service/handler` 四层实现，接口：`POST /api/v1/music-tasks`（创建，扣算力点）、`GET /api/v1/music-tasks`（列表，支持状态/分页）、`GET /api/v1/music-tasks/:id`（轮询详情）、`DELETE /api/v1/music-tasks/:id`、`POST /api/v1/music-tasks/lyrics`（歌词生成）。
- 后台轮询器（`service.StartMusicTaskPoller`，由 `main.go` 启动）按上游任务状态推进任务；**上游返回的音频会立即转存到自有存储**，任务只记录自有存储地址与 `mime_type/bytes/duration_ms`——上游地址只有 12 小时到 7 天有效期，不能透传给前端。
- 渠道协议适配层 `service/music_provider.go`：火山引擎豆包音乐走 API Gateway Bearer（`ServiceName: imagination`，`Version=2024-08-12`，默认用后付费 Action `GenSongForTime`/`GenBGMForTime`，预付费改常量即走 `GenSongV4`/`GenBGM`）；腾讯云 TokenHub 走同步接口并校验 `base_resp.status_code`。**AK/SK 的 HMAC-SHA256 模式未实现**，只支持 Gateway Bearer。
- 渠道 `Protocol` 取值 `volc-music` / `tokenhub-music`；`ModelCapabilities` 新增 `music` 能力值（音乐模型独立于 TTS 音频模型，避免与音色选择混在一起）。
- 新增 MiniMax 官方音乐协议 `minimax-music`（`POST /v1/music_generation`，同步返回；请求体、`base_resp.status_code` 校验与 `data.audio` 解析和 TokenHub 转售同构，两者共用同一套提交实现）。BaseURL 只填域名时补 `/v1/music_generation`，只填到 `/v1` 时补剩余路径，已带其它路径（第三方中转）时原样使用。歌词走官方 `POST /v1/lyrics_generation`（`mode: write_full_song`，取响应顶层 `lyrics`）。
- **上游服务调整**：MiniMax 官方自 2026 年 8 月 20 日起，音乐生成与歌词生成付费接口不再面向新用户提供服务（历史付费用户可继续使用），免费接口 `music-3.0-free` / `music-2.6-free` / `music-cover-free` 停止服务；新账号可能拿不到权限，属于上游策略，代码中已注释说明。
- 验证点：设置里能选到「MiniMax 音乐」渠道并填 `music-3.0` 等模型；`/music` 提交任务后同步返回并转存到自有存储；上游返回 `base_resp.status_code != 0` 时任务失败且退还算力点。
- 管理员 `GET /api/admin/tasks` 纳入音乐任务（`kind: "music"`），「我的任务」抽屉对管理员可见。
- 算力点：创建时扣费；**提交阶段就失败**（上游未接手）会退还，**上游已接手后失败不退还**（上游已产生消耗，代码中有明确注释说明这是刻意策略）。

**前端**
- `/music` 页面按原型实现：左侧 420px 参数区 + 右侧结果区；**参数区能力驱动**——切「歌曲/纯音乐」时时长区间、歌词输入、人声可用性随之变化并给出上限提示。
- 模型卡片显示**授权级别徽标**（官方 API / 授权转售）与折算单价及「预计消耗 N 算力点」（取真实的 `publicSettings.modelChannel.modelCosts`，算不出时显示 `—`）。
- 新增可复用音频播放器 `web/src/components/music/audio-player.tsx`（项目此前只有裸 `<audio controls>`）。
- 「我的音乐」历史、A/B 候选试听、合规声明（AI 标识与音频水印不可去除）。
- 无可用音乐模型时**优雅降级**：给出「请先在设置中配置音乐模型与渠道」并渲染一键打开配置弹窗的入口。
- 导航加回「音乐创作台」（位于「视频创作台」之后）；首页「我的图片」行下方新增「我的音乐」行（执行中卡片 + 已完成卡片，紧凑裸 `<audio>` 试听）；音乐任务接入右上角「我的任务」（`kind: music`，深链 `/music?task=<id>`）。

### 布局对齐「视频创作台」

按 [`video/page.tsx`](<web/src/app/(user)/video/page.tsx>) 重构 `/music` 的布局，左右两栏与该页**逐字同款**：

- **左右留白**：`main` 由「居中 + `max-w-7xl` + `px-6 py-6`」改为 **`p-3`（12px）+ 全宽 + grid 两列 `420px minmax(0,1fr)`**，与视频页一致。
- **左栏是单一面板**：固定 header（`text-2xl`「音乐创作台」+ 侧边/底部切换）→ 可滚动 body → **固定 footer**（生成按钮 + 算力点预估 + 错误提示），生成按钮不再随内容滚走。
- **历史记录**：由「参数卡片下方的独立卡片」移入**左栏滚动区内**的分组；底部布局下放入浮层展开区（`max-h-56` 可滚动），保证两种布局都有入口。
- **参数分组**：新增本地 `MusicSection`（照视频页 `WorkbenchSection`），分组为 创作模式 / 歌词 / 风格描述 / 风格标签 / 情绪场景 / 时长 / 参数 / 模型 / 生成数量 / 我的音乐。
- **侧边 / 底部布局切换**：新增 `WorkbenchLayout` 与 `MusicWorkbenchHeader`，偏好存 `localStorage`（键 `infinite-canvas:music-workbench-layout`，与视频页分开）。底部布局照视频页的**玻璃拟态浮层**（`fixed inset-x-0 bottom-5 z-40` + `rounded-[24px]` + `backdrop-blur-2xl`），结果面板加 `pb-40 lg:pb-44` 防遮挡。
- **右栏也是同款卡片面板**（`rounded-lg border bg-card shadow-sm lg:min-h-0 lg:overflow-y-auto lg:p-5`）：头部为图标 + `text-xl`「本次生成」+ 计数 `Tag` + 刷新；结果 `grid gap-3 md:grid-cols-2 2xl:grid-cols-3`；空态 `min-h-[320px] lg:min-h-[560px]` 虚线框。
- **提示词动作按钮**：歌词与风格描述各加「读取剪贴板」「清空」（歌词区另保留「AI 写词」）。**未加**「提示词库」「我的素材」——音乐暂无提示词分类，素材库对歌词/风格描述无意义。
- **A/B 候选**改为在各批次的**第一张结果卡片内部**展示。
- 修复：合规声明原被放进「有本次结果才渲染」的分支，导致**侧边布局空态下不显示**（底部布局是始终显示的，两边不一致）；现统一为**无论有无结果都显示**。

**已知取舍**：① 歌曲/纯音乐模式切换从 header 移到 body 第一个分组「创作模式」（420px 左栏里 header 同放标题+切换会换行）；② 底部浮层只放 模式/模型/时长/生成数量 + 风格标签 + 历史，**语言/人声/调式/BPM 需切回侧边布局**才能改；③ `canGenerate` 仍为「未选模型 / 提交中才禁用」，未收紧成「空输入即禁用」。

**验证情况（如实记录）**
- 已验证：`go build`/`go vet`/`go test` 全绿；前端 `tsc` 通过（仅 `model-picker.tsx` 4 条历史基线错误）；浏览器实测 11/11（导航项、首页行、`/music` 页面与模式切换）；用本地 mock 上游实跑通完整链路——创建扣费 → 轮询 16 次约 32 秒 → `completed`，`audio_url` 指向自有存储（`/api/files/<id>/content?s=<签名>`）且无 token 可播放，`storage_key`/`mime_type`/`bytes`/`duration_ms` 均已记录。
- **未验证**：真实厂商 API Key 的一次端到端（无 Key）；纯音乐 `GenBGMForTime` 分支与错误协议的路由拦截未跑完。
- **已补充验证（腾讯云 TokenHub）**：渠道 `channel-tokenhub-music`（协议 `tokenhub-music`，服务 ID `minimax-music-v2.6`，按量后付费）配置后，**从 `/music` 界面提交并真实生成成功**：`queued` → 约 88–120 秒 → `completed`，产物转存自有存储（`media/mpeg`，442–626 KB），浏览器实测可播（`accept-ranges: bytes`，`duration` 156 秒）。萤火山适配器的 `output_format` 曾固定 `hex`，**实测腾讯云网关在 hex 模式下 `data.audio` 返回空**，改为 `url` 后一次通过（上游 url 有效期 24 小时，服务端拿到后立即转存，无过期风险）。
- 布局对齐的运行时验证：左右面板 computed style 与视频页逐项一致（圆角 10px / 边框 1px / 内边距 / overflow）；结果网格在 1600px 宽下为 **3 列**；侧边↔底部切换、偏好持久化、浮层玻璃拟态（`blur(40px)` + `24px` 圆角）均实测通过。
- 已知配置要点：光配私有渠道不够，模型名还必须出现在公共配置的「系统可用模型」里，否则报「模型未开放」。

**待定**
- 公共配置缺少音乐的系统默认值（音频有 `DefaultAudioChannelID/Model/Voice` 三级默认，音乐没有），新用户需自行配置一次。
- 「重命名」「收藏」未实现（后端无对应接口，未自行编造）；A/B 候选为前端本地分组，刷新后分组信息丢失；任务抽屉里 music 与 audio 图标同为 `Music2`，建议区分。

## 隐藏没有提示词的分类

- `model.PromptCategory` 新增 `promptCount`（`gorm:"-"`，仅接口返回时填充），由新的 `repository.CountPromptsByCategory()` 按 `category` 聚合统计；`service.ListAllPromptCategories()` 统一补上计数。
- 提示词查询页面的分类列表（`service.ListPrompts` 返回的 `categories`，同时供视频/生图/画布里的提示词选择弹窗使用）现在会跳过 `promptCount == 0` 的分类。
- 管理页表格的「分类」筛选下拉同样只列出有提示词的分类。**「新增/编辑提示词」的分类选择与「同步」弹窗的远程分类列表仍保留全部分类**，否则会出现空分类无法补词、失败后无法手动补同步的问题。
- 验证点：管理页分类筛选下拉里不再出现「系统」；提示词查询页面的分类筛选从 14 项变为 13 项；新增/编辑弹窗与同步弹窗仍能看到全部分类。

## 提示词来源新增「清单聚合」类型与 YouMind 图像提示词

- 新增来源类型 `manifest`（`model.PromptSourceKindManifest`）：先从 URL 拉取清单 JSON，再按清单里的 `categories[].file` 逐个拉取分类文件并合并成一个来源，分类标题作为标签保留（提示词库支持按标签筛选）。实现见 `service/prompt_source.go` 的 `manifestPromptSourceItems`。
- 新增内置来源 `youmind-ai-image-prompts`（YouMind AI Image Prompts，仓库 `YouMind-OpenLab/ai-image-prompts-skill`）：清单里 11 个分类，实际同步 **22,908 条**（清单自报的 `totalPrompts: 15755` 与各分类 `count` 之和不一致，以分类文件为准），管理页「地址」列标注「清单聚合」，分类标题落成 11 个标签。
- 为兼容该仓库的数据格式：`promptSourceItem` 的 `id` 改为同时接受字符串与数字，新增 `sourceMedia` 作为封面回退字段；清单来源的提示词 ID 会带上分类前缀，避免不同分类文件之间 ID 冲突。
- 修复：提示词来源的「启用来源」开关对内置来源无效。管理页的开关与文案都表明可以切换，但 `SavePromptSource` 里的 `!source.BuiltIn` 判断把内置来源的开关静默忽略了。现在允许切换，内置来源仍不可改地址、不可删除。
- 修复：来源同步失败时不会记录原因，管理页永远显示「未同步」而不是「同步失败」；现在失败会把错误写进 `lastError`。
- 修复：`ReplacePromptSourcePrompts` 原来一次性 `Create` 全部条目，上万条会超出 SQLite 的单语句参数上限。改为按 500 行切块、每块一条独立 INSERT。
- 修复（重要）：来源同步在本项目的 SQLite 配置下**不能使用 GORM 的 `CreateInBatches`**。它会把所有批次放进同一个显式事务，实测在事务内执行第二条语句就会返回 `unable to open database file (14)`（服务端稳定复现：单批 200 行成功、500 行失败；改成单条语句后 2,131 行也能成功）。因此改为「按行数切块 + 每块一条独立语句」，不再使用 `CreateInBatches`。
- 调整：SQLite 连接池限制为单连接（`SetMaxOpenConns(1)`）。SQLite 同一时刻只允许一个写者，来源同步与后台轮询器、定时任务会并发写入。
- 拉取来源内容的 HTTP 客户端：总超时从 60 秒放宽到 20 分钟，TLS 握手超时从默认 10 秒放宽到 60 秒，响应头超时 120 秒，并对单次拉取自动重试 3 次（间隔 2 秒、4 秒）。该仓库 11 个分类合计约 47 MB，默认超时必然失败。
- 说明：`YouMind-OpenLab/awesome-gpt-image-2` 与 `YouMind-OpenLab/awesome-nano-banana-pro-prompts` 这两个仓库**不需要新增来源**——它们已经由「分类同步」路径（`buildPromptCategory` 直接解析仓库 `README_zh.md`，每天定时任务抓取）覆盖，对应分类 `youmind-gpt-image-2` 与 `youmind-nano-banana-pro`。对应的来源预设保持默认禁用，避免重复导入。
- 说明：`YouMind-OpenLab/ai-image-prompts-skill` 与 `YouMind-OpenLab/nano-banana-pro-prompts-recommend-skill` 的 11 个分类文件字节完全相同（manifest 只差 `updatedAt`），属于同一份数据，本次只接入前者。
- 验证点：管理页「提示词来源」出现 YouMind AI Image Prompts 且标注「清单聚合」、显示 22908 条与「已同步」；提示词库总数从 5757 变为 28665，按标签能筛出 11 个子分类；内置来源的启用开关切换后可以保存并生效。

## 我的任务

- 右上角头像**前面**新增「我的任务」图标入口（带执行中任务数量角标），头像本身不再挂角标，头像菜单里也去掉了该项；点开是右侧抽屉，列出执行中的生成任务（类型图标、提示词摘要、模型、进度条、已耗时、旋转图标与「执行中」状态）。没有执行中的任务时不显示该图标。
- 数据源在 `web/src/stores/use-task-store.ts`：视频创作台走 `GET /api/v1/video-tasks`，生图与工作流走后端图片任务列表，画布内的任务扫本地画布项目的节点元数据（`videoTaskId` / `imageTaskId` / `audioTaskId` 且节点状态为 `loading`）。有任务时跟随 `VIDEO_POLL_INTERVAL_MS`（5 秒）刷新，空闲时降到 15 秒。
- 点击任务回到来源页面并定位：视频 → `/video?task=<id>`，图片 → `/image?task=<id>`（复用页面已有的载入与高亮机制并滚动到位），画布任务 → `/canvas/<项目 id>?nodeId=<节点 id>`（复用 `focusNode` 平滑聚焦）。为此 `/video`、`/image`、`/canvas/[id]` 改为 `Suspense` + `useSearchParams` 读取参数。
- 管理员登录时改为调用新增的 `GET /api/admin/tasks`，列出全部用户的执行中任务并标注用户名。其中**属于管理员自己**的任务与普通用户一样可点击跳转（`/video?task=`、`/image?task=`）；其他用户的任务属于各自浏览器里的项目，只展示不跳转。
- 首页「我的画布 / 我的视频 / 我的图片」三行把执行中任务排在各自行首，用带脉冲底纹、旋转图标、进度条和「执行中」标签的卡片展示。
- 验证点：创建视频生成任务后切到其他路由，头像出现角标；点「我的任务」能看到该任务与进度；点击后回到 `/video` 并高亮定位该条记录；首页对应行首出现执行中卡片且动画在动；画布内发起的任务归到「我的画布」行，点击回到画布并聚焦到对应节点；管理员账号能看到其他用户的任务与用户名。

## 对象下载鉴权与历史地址回填

- `GET /api/files/:id/content` 现在要求签名参数 `?s=<HMAC>`（实现见 `service/storage.go`，密钥取自 `JWT_SECRET`）；`GET /api/files/:id` 挂到 `middleware.UserAuth` 并校验对象归属（`created_by` 为本人或管理员，`anonymous` 与空值视为无归属）。此前只要知道对象 UUID 就能下载任意用户的图片/视频/音频。
- 上传与直传登记接口返回的地址、以及 `GET /api/files/:id` 新增的 `contentUrl` 字段都是签名地址；前端 `resolveMediaUrl` / `resolveImageUrl` 优先使用 `contentUrl`，并且不再对 `/api/files/` 开头的旧地址直接短路返回。
- 一次性回填脚本 `scripts/backfill-storage-urls`：默认只统计，加 `-apply` 才写库，`-verify` 只读扫描全部表并列出仍含未签名地址的表与字段。本机数据库已执行，`-verify` 结果为 0。
- 验证点：画布里的历史图片、视频、音频仍能显示与播放；两个工作台的历史缩略图正常；不带 `s` 参数直接访问 `/api/files/<id>/content` 会失败；用其他账号的 token 调 `/api/files/<id>` 返回无权访问。

## 提示词详情预览视频的播放停止

- 现象：提示词管理（`/admin/prompts`）里点开带视频的提示词，关闭弹框后视频仍在播放。
- 原因：antd `Modal` 默认不销毁内容，关闭只是把 `.ant-modal-wrap` 置为 `display: none`；弹框里的 `<video autoPlay loop>` 仍挂在 DOM 上且 `src` 未清空，是否停止播放完全交给浏览器对隐藏元素的处理。此外切换提示词时 `videoFailed` 状态不会重置，一条视频加载失败后后续视频都会退化成封面。
- 改动：详情弹框加 `destroyOnHidden`，关闭时真正卸载内容；`PromptPreview`（`web/src/components/prompts/prompt-cover.tsx`）用 ref 在卸载或切换视频时显式 `pause()` 并清空 `src` 后 `load()`，不再依赖浏览器行为。
- 验证：`scripts/e2e/prompt_video_tests.py` 覆盖底部「关闭」按钮、右上角 X、ESC 三种关闭方式，断言关闭前视频处于播放状态、关闭后页面已无 `<video>` 元素且原播放器已暂停。修复前 15/18 通过（关闭后 `<video>` 仍留在 DOM 中），修复后 18/18 通过。

## 自动化功能测试（admin + guoguogis）

- 测试脚本放在 `scripts/e2e/`：`api_tests.py`（接口层）、`ui_tests.py`（Playwright + Chrome 浏览器层）、`prompt_video_tests.py`（提示词视频预览回归），夹具工具为 `scripts/e2e/testctl`（`seed` / `clean` / `stats`）。账号密码通过 `IC_E2E_ADMIN_PASS`、`IC_E2E_USER_PASS` 环境变量传入，不写进仓库；用户 id 由登录接口返回，不硬编码。
- 接口层覆盖：未登录访问用户侧接口一律 401；画布项目、视频/图片生成记录、视频任务、图片任务按账号隔离；跨账号写入与跨账号删除互不影响；`/api/admin/*` 对普通用户 401、对管理员可用；对象下载缺签名/错签名被拒、正确签名可下载、他人读取被拒、归属者可读；`/api/admin/tasks` 返回全部用户任务且可按 `userId` 过滤并标注用户名。结果 63 项全部通过。
- 浏览器层覆盖：未登录访问 `/canvas` 跳转登录页并带回 `redirect`；登录后浏览器 IndexedDB 中业务数据落在 `u_<用户id>__*` 命名空间，且不存在另一个账号的命名空间；首页只显示自己的画布、看不到对方画布；头像角标与「我的任务」抽屉数量、内容正确；点击执行中任务跳转到 `/video?task=<id>` 并高亮定位该记录；`/canvas/<项目id>?nodeId=<节点id>` 能渲染并定位目标节点；同一浏览器 profile 切换账号后两个命名空间互不重叠且各自数据仍在；管理员抽屉能看到其他账号的任务。结果 39 项全部通过。
- 本地存储命名空间在首次渲染前会有一个 `guest__app_state`，以及 localforage 自身的 `local-forage-detect-blob-support` 探测库，两者都不承载业务数据。
- 运行时观察（非本次引入）：顶栏「音乐创作台」指向 `/music`，但项目没有该路由，页面会请求到一个 404 的 RSC 预取；现已把该导航项从 `web/src/constant/navigation-tools.ts` 移除，桌面导航与移动端抽屉同时不再显示（路由本身仍未实现，后续补页面时再加回导航项）。


