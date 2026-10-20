package pages

import (
	"fmt"
	"strings"
)

// CurrentBackupVersion 当前备份/配置文件格式版本。
//
// 变更约定（重要）：
//   - 增删字段、改语义、改默认值 → 必须 +1
//   - 导入时比对：备份版本 > 当前 → 拒绝（旧程序读不懂新数据）
//   - 备份版本 < 当前 → 走 migrateBackup 逐级升级
//   - 相等 → 直接用
//
// 版本历史：
//
//	1  初始版本（groups + cards 整体替换）
const CurrentBackupVersion = 1

// 备份版本校验结果
type backupCheckResult struct {
	OK      bool   // 是否允许继续导入
	Migrate bool   // 是否需要迁移（备份版本低于当前）
	From    int    // 备份声明的版本
	Reason  string // 不允许时的原因（面向用户）
}

// checkBackupVersion 校验备份版本。
//
// 设计要点：
//   - 备份未声明 version（老备份，字段缺失 → 0）按版本 1 处理，
//     因为初始版本导出时就已经写了 version:1，缺失说明是更早的手工产物，
//     其结构与 v1 兼容，容忍而非拒绝。
//   - 备份版本高于当前：拒绝。旧程序无法理解新字段，
//     强行导入会静默丢数据，这比报错更危险。
func checkBackupVersion(from int) backupCheckResult {
	if from <= 0 {
		// 缺失或非法值，按 v1 处理（向后兼容）
		from = 1
	}
	if from > CurrentBackupVersion {
		return backupCheckResult{
			OK:     false,
			From:   from,
			Reason: fmt.Sprintf("该备份版本（v%d）高于当前程序支持的版本（v%d），请先升级程序后再导入。", from, CurrentBackupVersion),
		}
	}
	if from < CurrentBackupVersion {
		return backupCheckResult{
			OK:      true,
			Migrate: true,
			From:    from,
			Reason:  fmt.Sprintf("备份版本 v%d 低于当前版本 v%d，将自动升级数据格式。", from, CurrentBackupVersion),
		}
	}
	return backupCheckResult{OK: true, From: from}
}

// normalizeBackupVersion 把 JSON 里可能出现的多种类型统一成 int。
//
// 备份文件是人手可编辑的 JSON，version 可能被写成 1 / "1" / 1.0。
// 用 json.Number 解析后转 int，比直接断言 float64 更稳。
func normalizeBackupVersion(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return 0
		}
		n := 0
		neg := false
		for i, ch := range x {
			if i == 0 && ch == '-' {
				neg = true
				continue
			}
			if ch < '0' || ch > '9' {
				return 0 // 含非数字，视为无效
			}
			n = n*10 + int(ch-'0')
		}
		if neg {
			return 0
		}
		return n
	}
	return 0
}

// migrateBackup 把低版本备份升级到当前版本。
//
// 当前只有 v1，无实际迁移动作。保留此函数是为了：
//  1. 未来加 v2 时，迁移逻辑有明确落点，不用改调用方
//  2. 让"迁移"这件事在代码里显式存在，而不是散落各处
//
// 新增版本时在此按 from 逐级向上补：
//
//	if from < 2 { /* v1 → v2 的字段补齐 */ from = 2 }
func migrateBackup(from int, payload *backupPayload) error {
	if from <= 0 {
		from = 1
	}
	// v1 无需迁移
	_ = from
	_ = payload
	return nil
}
