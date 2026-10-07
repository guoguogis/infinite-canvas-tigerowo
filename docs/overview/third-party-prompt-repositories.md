---
title: 第三方 GitHub 提示词仓库
description: 当前已接入同步逻辑的第三方提示词仓库
---

# 第三方 GitHub 提示词仓库

| 地址 | 状态 |
| --- | --- |
| https://github.com/EvoLinkAI/awesome-gpt-image-2-API-and-Prompts | 已实现同步逻辑 |
| https://github.com/ZeroLu/awesome-gpt-image | 已实现同步逻辑 |
| https://github.com/ImgEdify/Awesome-GPT4o-Image-Prompts | 已实现同步逻辑 |
| https://github.com/xianyu110/awesome-gptimage2 | 已实现同步逻辑 |
| https://github.com/YouMind-OpenLab/awesome-gpt-image-2 | 已实现同步逻辑 |
| https://github.com/YouMind-OpenLab/awesome-nano-banana-pro-prompts | 已实现同步逻辑 |
| https://github.com/davidwuw0811-boop/awesome-gpt-image2-prompts | 已实现同步逻辑 |

## 提示词来源

提示词管理页的「提示词来源」可以新增自定义来源：填写来源名称与一个返回提示词数组的 JSON 地址，同步后提示词会以来源名作为分类进入提示词库，画布侧栏和提示词页都能直接使用。内置来源支持单独同步、启停，非内置来源可以编辑和删除（删除时会一并清理它同步进来的提示词）。

启用的来源会出现在提示词管理页的分类筛选、提示词页和画布侧栏的分类列表里，内置视频来源排在最前；在来源分类下手动新增或编辑提示词会保留该分类，未知分类仍然回落到默认分类。

除上表的独立仓库外，还会预置一批来自提示词注册表的来源：

| 来源 | 地址前缀 | 状态 |
| --- | --- | --- |
| Banana Prompt Quicker | `yukkcat/image-prompts` 的 `dist/sources` | 内置，默认启用 |
| Freestylefly GPT Image 2 | 同上 | 内置，默认启用 |
| Awesome GPT Image | 同上 | 内置，默认停用（与上表分类重复） |
| Awesome GPT4o Image Prompts | 同上 | 内置，默认停用（与上表分类重复） |
| YouMind GPT Image 2 | 同上 | 内置，默认停用（与上表分类重复） |
| YouMind Nano Banana Pro | 同上 | 内置，默认停用（与上表分类重复） |
| DavidWu GPT Image 2 | 同上 | 内置，默认停用（与上表分类重复） |

JSON 结构为数组，每项至少包含标题与提示词正文；字段名兼容 `title`/`name`、`prompt`/`content`、`coverUrl`/`cover`/`image` 等常见别名，另可选 `videoUrl`/`videoUrls`、`tags`、`preview`、`author`、`createdAt`、`updatedAt`。

### 内置视频提示词来源

视频来源的数据由仓库内的生成脚本按上面的 JSON 格式产出并内置到程序里，同步时直接读取，不依赖外网：

| 来源 | 条数 | 数据来源 |
| --- | --- | --- |
| MiniMax H3 Prompt Library | 85 | https://www.atlascloud.ai/prompts-hub/minimax-h3-prompt 页面内嵌数据 |
| Seedance 2.0 Prompts | 2706 | https://github.com/AtlasCloudAI/awesome-seedance-2-prompts 的 `data/prompts.json` |
| Lanshu AI Video Kit | 494 | https://github.com/cclank/lanshu-awesome-ai-video-kit 的 `prompts/data/all-prompts.json` |

内置数据以 gzip + base64 形式存放在 `service/prompt_source_builtin_data.go`，由 `scripts/prompt-sources/generate.mjs` 生成：

```bash
node scripts/prompt-sources/generate.mjs
```

上游更新后重新执行生成脚本并重新编译后端，再在提示词来源页点同步即可；内置来源不支持编辑地址和删除。

封面与视频：MiniMax H3 使用上游 `remote_images` 作为封面并保存 `remote_videos` 作为示例视频，缺失图片时回退到视频首帧；Seedance 使用视频首帧作为封面并保存原视频；Lanshu 上游没有任何图片或视频字段，因此既无封面也无示例视频。示例视频只在管理后台 `/admin/prompts` 中用于预览（封面显示播放标识，详情弹窗内播放）；客户视图只展示封面图，不播放视频。
