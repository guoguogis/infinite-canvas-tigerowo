---
title: TODO
description: 当前项目后续值得处理的事项
---

# TODO

本文档用来记录当前项目后续比较值得处理的事项。

## 待办

- 「音乐创作台」（`/music`）已从 `web/src/constant/navigation-tools.ts` 的导航项里移除，但项目仍没有该路由；等功能做完再把导航项加回去。
- 没有可用对象存储 Provider 时给出可读的降级提示：目前 `selectStorageProvider` 会直接报「没有可用对象存储配置」，上传、云端同步与 `/api/files/:id/content` 都不可用，但生成功能仍可用，界面上没有对应说明，云端存储相关入口也没有置灰。
- `/api/v1/videos/:id/content` 在数据库未命中本地任务时会回落到直连上游代理（`handler/ai.go`），该分支没有归属校验，上游若不校验任务归属，登录用户凭已知的上游 video id 可能取到他人任务内容。
- `services/storage-migration.ts` 已无任何调用方（`checkLocalAssetsExist`、`migrateLocalAssetsToCloud` 均为死代码），确认不需要后可删除。
- `assets` 素材库仍是全局共享表（无用户字段，只由管理员维护），用户侧 `GET /api/assets` 会返回全部素材；如果后续要让每个用户维护自己的素材库，需要先补归属字段。

