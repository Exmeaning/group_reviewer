# group_reviewer —— ZeroBot 进群申请审核插件

## 功能概述

| 功能 | 说明 |
|------|------|
| 📋 申请转发 | 监控群 `1075068454` 的进群申请，自动推送至审核群 `1095426209` |
| 👤 详细信息 | 推送昵称、QQ号、等级、头像、进群理由、Flag |
| ✅ `/同意` | 审核群成员发指令，允许该用户进群 |
| ❌ `/拒绝` | 审核群成员发指令，拒绝进群 |
| 🚫 `/黑名单` | 拒绝进群且永久拉黑（`reject_add_request=true`） |
| ⚡ 自动同意 | 等级 > 25 且进群理由含 `worlders`（不区分大小写），12h 无人处理则自动同意 |

---

## 文件结构

```
group_reviewer/
├── group_reviewer.go   # 插件主体
└── README.md           # 本说明文档
```

---

## 快速接入

### 1. 将插件放入你的 ZeroBot 项目

```bash
cp -r group_reviewer/ /path/to/your/zerobot-project/plugin/
```

### 2. 在 `main.go` 中导入插件

```go
import (
    _ "your-module/plugin/group_reviewer"
    // ... 其他插件
)
```

### 3. 确认配置文件中 Bot 是监控群管理员

Bot 账号必须是群 `1075068454` 的**管理员**，才能：
- 收到进群申请事件
- 执行同意/拒绝操作

---

## 使用说明

### 审核群收到的消息格式

```
📋 【新进群申请】
━━━━━━━━━━━━━━━━━━━
👤 昵称：某某某
🆔 QQ号：123456789
⭐ 等级：30
💬 进群理由：I am a worlders member
🖼  头像：https://q1.qlogo.cn/...
🔑 Flag：xxxxxxxxxxxxxxxx
━━━━━━━━━━━━━━━━━━━
⚡ 自动同意提示：该用户等级 30 > 25 且申请理由含关键词「worlders」
⏰ 若 12 小时内无人处理，将自动允许进群！
📌 操作指令（在本群发送）：
  /同意 xxxxxxxxxxxxxxxx
  /拒绝 xxxxxxxxxxxxxxxx
  /黑名单 xxxxxxxxxxxxxxxx
```

### 指令格式

```
/同意 <flag>        # 允许进群
/拒绝 <flag>        # 拒绝进群
/黑名单 <flag>      # 拒绝进群 + 永久拉黑
```

---

## 自动同意逻辑流程

```
收到进群申请
    │
    ├─ 等级 > 25 AND 理由含 "worlders" ?
    │       │
    │      YES ──→ 消息中附加自动同意提示
    │               │
    │               └─ 启动 12h 定时器
    │                       │
    │                 12h 内有人 /同意、/拒绝、/黑名单
    │                       │         │
    │                    定时器取消   无人操作
    │                               │
    │                          自动同意进群
    │                          通知审核群
    │
    NO ──→ 仅等待人工处理（无自动操作）
```

---

## 注意事项

1. **等级字段**：`GetStrangerInfo` 返回的 `level` / `qq_level` 字段因实现（go-cqhttp / NapCat / Lagrange）而异，插件已做兼容处理。

2. **黑名单实现**：`set_group_kick` + `reject_add_request=true` 是 OneBot11 标准拉黑方式。申请人此时未入群，部分实现（如 go-cqhttp）支持对非群员也记录拉黑，具体效果取决于你的后端。

3. **头像 URL**：使用标准 QQ 头像 CDN `https://q1.qlogo.cn/g?b=qq&nk={QQ}&s=640`，无需 API 调用。

4. **并发安全**：所有 `pending` 状态操作通过 `sync.Mutex` 保护，定时器回调与人工指令互斥，不会重复处理。

5. **重启丢失**：当前使用内存 map 存储申请，Bot 重启后未处理的申请会丢失（定时器也会失效）。如需持久化，可扩展为 SQLite 存储。

---

## 依赖

- [ZeroBot](https://github.com/wdvxdr1123/ZeroBot) v1.7+
- OneBot11 协议后端（go-cqhttp / NapCat / Lagrange 等）

---

## 修改常量

在 `group_reviewer.go` 顶部修改以下常量即可：

```go
const (
    TargetGroupID             int64  = 1075068454  // 被监控群
    ReviewGroupID             int64  = 1095426209  // 审核群
    AutoApproveHours                 = 12          // 自动同意等待时长（小时）
    AutoApproveLevelThreshold        = 25          // 触发自动同意的最低等级
    AutoApproveKeyword               = "worlders"  // 关键词（大小写不敏感）
)
```
