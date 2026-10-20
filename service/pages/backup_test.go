package pages

import (
	"testing"
)

// TestCheckBackupVersion 覆盖版本校验的各种输入。
//
// 这是导入流程的安全闸门，必须严格：
//   - 高于当前 → 拒绝（否则静默丢数据）
//   - 低于当前 → 允许 + 标记迁移
//   - 缺省/非法 → 容忍为 v1（向后兼容）
func TestCheckBackupVersion(t *testing.T) {
	cur := CurrentBackupVersion

	// 注意：from=0 会被归一化为 v1（向后兼容）。
	// 因此"低于当前且有效"的版本只在 cur>1 时存在。
	// 这个分支要随 CurrentBackupVersion 变化自动成立，不能写死。
	lower := cur - 1
	lowerIsValid := lower >= 1

	tests := []struct {
		name        string
		from        int
		wantOK      bool
		wantMigrate bool
	}{
		{"当前版本-允许", cur, true, false},
		{"高于当前-拒绝", cur + 1, false, false},
		{"远高于当前-拒绝", cur + 99, false, false},
		{"零值-按v1容忍", 0, true, false},
		{"负数-按v1容忍", -5, true, false},
	}

	// 仅当存在合法的更低版本时才加这条用例
	if lowerIsValid {
		tests = append(tests, struct {
			name        string
			from        int
			wantOK      bool
			wantMigrate bool
		}{"低于当前-允许并迁移", lower, true, true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBackupVersion(tt.from)
			if got.OK != tt.wantOK {
				t.Errorf("checkBackupVersion(%d).OK = %v, 期望 %v", tt.from, got.OK, tt.wantOK)
			}
			if got.Migrate != tt.wantMigrate {
				t.Errorf("checkBackupVersion(%d).Migrate = %v, 期望 %v", tt.from, got.Migrate, tt.wantMigrate)
			}
			// 拒绝时必须给出可读原因，否则前端无从提示
			if !got.OK && got.Reason == "" {
				t.Errorf("checkBackupVersion(%d) 被拒绝但未提供 Reason", tt.from)
			}
		})
	}
}

// TestCheckBackupVersion_ZeroNormalizedToV1 明确锁定"缺失版本按 v1 处理"这一约定。
// 这条约定决定了老备份能否被导入，属于兼容性契约，必须显式固定。
func TestCheckBackupVersion_ZeroNormalizedToV1(t *testing.T) {
	got := checkBackupVersion(0)
	// 期望：0 → 视为 v1 → 与当前比较
	expectOK := CurrentBackupVersion >= 1
	if got.OK != expectOK {
		t.Errorf("version=0 应视为 v1；OK=%v，期望 %v", got.OK, expectOK)
	}
	if got.From != 1 {
		t.Errorf("version=0 归一化后 From 应为 1，实际 %d", got.From)
	}
}

// TestNormalizeBackupVersion 备份文件可手工编辑，version 类型必须容错。
func TestNormalizeBackupVersion(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int
	}{
		{"float64-JSON默认", float64(1), 1},
		{"float64-小数被截断", float64(2.9), 2},
		{"int", int(3), 3},
		{"字符串数字", "4", 4},
		{"字符串带空格", "  5  ", 5},
		{"空字符串", "", 0},
		{"非数字字符串", "abc", 0},
		{"混入非数字", "1a", 0},
		{"负数", "-1", 0},
		{"nil", nil, 0},
		{"bool-不支持", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBackupVersion(tt.in); got != tt.want {
				t.Errorf("normalizeBackupVersion(%#v) = %d, 期望 %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestMigrateBackup 当前只有 v1，迁移应为空操作且不报错。
// 未来加 v2 时，这里补上对应的字段断言。
func TestMigrateBackup(t *testing.T) {
	p := &backupPayload{}
	for _, from := range []int{0, 1} {
		if err := migrateBackup(from, p); err != nil {
			t.Errorf("migrateBackup(%d) 返回错误: %v", from, err)
		}
	}
}
