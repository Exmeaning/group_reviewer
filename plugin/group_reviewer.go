// Package group_reviewer 进群申请审核插件
// 监听群 1075068454 的进群申请，转发至群 1095426209 进行人工审核
// 支持 /同意 /拒绝 /黑名单 指令，以及等级>25且含关键词"worlders"时12h自动同意
package plugin

import (
	"fmt"
	"strings"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// ─────────────────────────────────────────────
// 常量配置
// ─────────────────────────────────────────────

const (
	// TargetGroupID 被监控群（产生进群申请的群）
	TargetGroupID int64 = 1075068454
	// ReviewGroupID 审核群（接收通知并下发指令的群）
	ReviewGroupID int64 = 1095426209
	// AutoApproveHours 等级与关键词双条件满足时，自动同意等待时间（小时）
	AutoApproveHours = 12
	// AutoApproveLevelThreshold 触发自动同意所需的最低等级（严格大于）
	AutoApproveLevelThreshold = 25
	// AutoApproveKeyword 触发自动同意的关键词（大小写不敏感）
	AutoApproveKeyword = "worlders"
)

// ─────────────────────────────────────────────
// 申请记录结构体
// ─────────────────────────────────────────────

// PendingRequest 保存每条待审核的进群申请信息
type PendingRequest struct {
	Flag        string      // OneBot flag，处理申请的唯一凭据
	SubType     string      // add / invite
	UserID      int64       // 申请人 QQ 号
	Nickname    string      // 申请人昵称
	Level       int64       // 申请人等级（来自陌生人信息）
	AvatarURL   string      // 头像 URL
	Comment     string      // 进群理由
	AutoApprove bool        // 是否命中自动同意条件
	Timer       *time.Timer // 自动同意定时器（仅 AutoApprove=true 时有效）
	Handled     bool        // 是否已被人工/自动处理（防止重复触发）
}

// ─────────────────────────────────────────────
// 全局状态（flag → PendingRequest）
// ─────────────────────────────────────────────

var (
	pendingMu sync.Mutex
	pending   = make(map[string]*PendingRequest)
)

// ─────────────────────────────────────────────
// 插件注册入口
// ─────────────────────────────────────────────

func init() {
	engine := zero.New()

	// ── 1. 监听进群申请事件（仅监控群） ─────────
	engine.On("request/group/add", onlyTargetGroup).
		SetBlock(false).
		Handle(onGroupAddRequest)

	// ── 2. /同意 <flag>（仅审核群） ─────────────
	engine.OnCommand("同意",
		zero.OnlyGroup,
		zero.CheckGroup(ReviewGroupID),
	).SetBlock(true).Handle(onApprove)

	// ── 3. /拒绝 <flag>（仅审核群） ─────────────
	engine.OnCommand("拒绝",
		zero.OnlyGroup,
		zero.CheckGroup(ReviewGroupID),
	).SetBlock(true).Handle(onReject)

	// ── 4. /黑名单 <flag>（仅审核群） ───────────
	engine.OnCommand("黑名单",
		zero.OnlyGroup,
		zero.CheckGroup(ReviewGroupID),
	).SetBlock(true).Handle(onBlacklist)
}

// ─────────────────────────────────────────────
// Rule：仅处理来自被监控群的申请
// ─────────────────────────────────────────────

func onlyTargetGroup(ctx *zero.Ctx) bool {
	return ctx.Event.GroupID == TargetGroupID
}

// ─────────────────────────────────────────────
// Handler：收到进群申请
// ─────────────────────────────────────────────

func onGroupAddRequest(ctx *zero.Ctx) {
	flag := ctx.Event.Flag
	userID := ctx.Event.UserID
	subType := ctx.Event.SubType // "add" 或 "invite"
	comment := strings.TrimSpace(ctx.Event.Comment)

	// ── 获取陌生人信息（昵称、等级） ──────────────
	stranger := ctx.GetStrangerInfo(userID, false)
	nickname := stranger.Get("nickname").String()
	level := stranger.Get("level").Int()
	// 部分实现（如 NapCat）用 qq_level 字段暴露等级
	if level == 0 {
		level = stranger.Get("qq_level").Int()
	}
	// 头像 URL 直接拼接，无需额外 API 调用
	avatarURL := fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", userID)

	// ── 判断是否命中自动同意条件 ──────────────────
	autoApprove := level > AutoApproveLevelThreshold &&
		strings.Contains(strings.ToLower(comment), strings.ToLower(AutoApproveKeyword))

	// ── 构建并存储申请记录 ────────────────────────
	req := &PendingRequest{
		Flag:        flag,
		SubType:     subType,
		UserID:      userID,
		Nickname:    nickname,
		Level:       level,
		AvatarURL:   avatarURL,
		Comment:     comment,
		AutoApprove: autoApprove,
	}

	pendingMu.Lock()
	pending[flag] = req
	pendingMu.Unlock()

	// ── 构建发往审核群的消息 ──────────────────────
	sep := "━━━━━━━━━━━━━━━━━━━"
	msgLines := []string{
		"📋 【新进群申请】",
		sep,
		fmt.Sprintf("👤 昵称：%s", nickname),
		fmt.Sprintf("🆔 QQ号：%d", userID),
		fmt.Sprintf("⭐ 等级：%d", level),
		fmt.Sprintf("💬 进群理由：%s", ifEmpty(comment, "（未填写）")),
		fmt.Sprintf("🖼  头像：%s", avatarURL),
		fmt.Sprintf("🔑 Flag：%s", flag),
		sep,
	}

	if autoApprove {
		msgLines = append(msgLines,
			fmt.Sprintf("⚡ 自动同意提示：等级 %d > %d 且理由含关键词「%s」",
				level, AutoApproveLevelThreshold, AutoApproveKeyword),
			fmt.Sprintf("⏰ 若 %d 小时内无人处理，将自动允许进群！", AutoApproveHours),
			sep,
		)
	}

	msgLines = append(msgLines,
		"📌 操作指令（在本群发送）：",
		fmt.Sprintf("  /同意 %s", flag),
		fmt.Sprintf("  /拒绝 %s", flag),
		fmt.Sprintf("  /黑名单 %s", flag),
	)

	fullMsg := strings.Join(msgLines, "\n")

	// ── 发送头像图片 + 文字到审核群 ───────────────
	chain := message.Message{
		message.Image(avatarURL),
		message.Text("\n" + fullMsg),
	}
	ctx.SendGroupMessage(ReviewGroupID, chain)

	// ── 启动自动同意定时器（条件满足时）──────────
	if autoApprove {
		// 闭包捕获不变量，避免 ctx 被 GC
		capturedFlag := flag
		capturedSubType := subType
		capturedNickname := nickname
		capturedUserID := userID
		capturedLevel := level

		timer := time.AfterFunc(AutoApproveHours*time.Hour, func() {
			pendingMu.Lock()
			r, ok := pending[capturedFlag]
			if !ok || r.Handled {
				pendingMu.Unlock()
				return
			}
			r.Handled = true
			delete(pending, capturedFlag)
			pendingMu.Unlock()

			// 自动同意
			ctx.SetGroupAddRequest(capturedFlag, capturedSubType, true, "")

			// 通知审核群
			ctx.SendGroupMessage(ReviewGroupID, message.Message{
				message.Text(fmt.Sprintf(
					"✅ [自动同意] 用户 %s(%d) 的进群申请已自动同意\n"+
						"（等级 %d > %d，理由含「%s」，超过 %dh 无人处理）\n"+
						"Flag: %s",
					capturedNickname, capturedUserID,
					capturedLevel, AutoApproveLevelThreshold,
					AutoApproveKeyword, AutoApproveHours,
					capturedFlag,
				)),
			})
		})

		pendingMu.Lock()
		if r, ok := pending[flag]; ok {
			r.Timer = timer
		}
		pendingMu.Unlock()
	}
}

// ─────────────────────────────────────────────
// Handler：/同意 <flag>
// ─────────────────────────────────────────────

func onApprove(ctx *zero.Ctx) {
	flag := strings.TrimSpace(ctx.State["args"].(string))
	if flag == "" {
		ctx.SendChain(message.Text("❌ 用法：/同意 <flag>"))
		return
	}

	req, ok := getAndMarkHandled(flag)
	if !ok {
		ctx.SendChain(message.Text("⚠️ 未找到对应申请，可能已被处理或过期\nFlag: " + flag))
		return
	}

	ctx.SetGroupAddRequest(flag, req.SubType, true, "")
	ctx.SendChain(message.Text(fmt.Sprintf(
		"✅ 已同意 %s(%d) 的进群申请\nFlag: %s",
		req.Nickname, req.UserID, flag,
	)))
}

// ─────────────────────────────────────────────
// Handler：/拒绝 <flag>
// ─────────────────────────────────────────────

func onReject(ctx *zero.Ctx) {
	flag := strings.TrimSpace(ctx.State["args"].(string))
	if flag == "" {
		ctx.SendChain(message.Text("❌ 用法：/拒绝 <flag>"))
		return
	}

	req, ok := getAndMarkHandled(flag)
	if !ok {
		ctx.SendChain(message.Text("⚠️ 未找到对应申请，可能已被处理或过期\nFlag: " + flag))
		return
	}

	ctx.SetGroupAddRequest(flag, req.SubType, false, "申请已被拒绝")
	ctx.SendChain(message.Text(fmt.Sprintf(
		"❌ 已拒绝 %s(%d) 的进群申请\nFlag: %s",
		req.Nickname, req.UserID, flag,
	)))
}

// ─────────────────────────────────────────────
// Handler：/黑名单 <flag>
// ─────────────────────────────────────────────

func onBlacklist(ctx *zero.Ctx) {
	flag := strings.TrimSpace(ctx.State["args"].(string))
	if flag == "" {
		ctx.SendChain(message.Text("❌ 用法：/黑名单 <flag>"))
		return
	}

	req, ok := getAndMarkHandled(flag)
	if !ok {
		ctx.SendChain(message.Text("⚠️ 未找到对应申请，可能已被处理或过期\nFlag: " + flag))
		return
	}

	// 拒绝进群申请
	ctx.SetGroupAddRequest(flag, req.SubType, false, "已加入黑名单")

	// 调用封装好的 SetGroupKick（rejectAddRequest=true 即永久拉黑）
	// 申请人未入群，部分后端（go-cqhttp/NapCat）支持对非群员记录拉黑
	ctx.SetGroupKick(TargetGroupID, req.UserID, true)

	ctx.SendChain(message.Text(fmt.Sprintf(
		"🚫 已拒绝并将 %s(%d) 加入黑名单\nFlag: %s",
		req.Nickname, req.UserID, flag,
	)))
}

// ─────────────────────────────────────────────
// 工具函数
// ─────────────────────────────────────────────

// getAndMarkHandled 取出申请记录并标记为已处理，同时停止自动定时器。
// 若记录不存在或已被处理，返回 (nil, false)。
func getAndMarkHandled(flag string) (*PendingRequest, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	req, ok := pending[flag]
	if !ok || req.Handled {
		return nil, false
	}

	req.Handled = true
	if req.Timer != nil {
		req.Timer.Stop()
		req.Timer = nil
	}
	delete(pending, flag)
	return req, true
}

// ifEmpty 若 s 为空则返回 fallback
func ifEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
