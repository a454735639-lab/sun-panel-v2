package panel

import (
	"strings"
	"testing"
)

// 分组更新的字段白名单。
//
// 背景：Update 用 `Select(updateField)` 显式指定要写的列，不在列表里的字段
// 即便请求带了也不会落库。CardStyle 曾因此长期失效 ——
// 模型有、数据库列有、前端也提交了，唯独更新语句没带上。
//
// 这类问题的可怕之处是**静默**：接口返回 code=0「保存成功」，
// 但值根本没变，用户只会觉得"改了没反应"。
//
// 下面用源码文本断言守住这条约束。虽然不如集成测试严格，
// 但能在改动时立刻报警，且不依赖数据库环境。

// TestGroupUpdateIncludesCardStyle 白名单必须含 CardStyle
func TestGroupUpdateIncludesCardStyle(t *testing.T) {
	src := readSource(t, "ItemIconGroup.go")

	// 定位 updateField 的赋值段
	idx := strings.Index(src, "updateField := []string{")
	if idx < 0 {
		t.Fatal("找不到 updateField 定义，代码结构可能已变，请更新此测试")
	}
	// 取到下一个 } 为止
	rest := src[idx:]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal("updateField 定义未闭合")
	}
	block := rest[:end]

	if !strings.Contains(block, "CardStyle") {
		t.Errorf("updateField 缺少 CardStyle —— 分组卡片风格将无法保存。\n当前块:\n%s", block)
	}
}

// TestGroupUpdateRequiresOwnership 更新必须带 user_id 条件
func TestGroupUpdateRequiresOwnership(t *testing.T) {
	src := readSource(t, "ItemIconGroup.go")

	if !strings.Contains(src, `"id = ? AND user_id = ?"`) {
		t.Error("分组更新缺少 user_id 条件 —— 存在越权改他人分组的风险")
	}
}

// TestCardItemUpdateIncludesWidgetAndCardType 卡片更新的白名单
//
// 同类问题：WidgetType / WidgetConfig 与 CardType / ExpandParam
// 都曾因不在白名单而无法保存（小组件配了没反应、老卡片改不成 Docker 卡）。
func TestCardItemUpdateIncludesWidgetAndCardType(t *testing.T) {
	src := readSource(t, "itemIcon.go")

	idx := strings.Index(src, "updateField := []string{")
	if idx < 0 {
		t.Fatal("找不到 updateField 定义")
	}
	rest := src[idx:]
	end := strings.Index(rest, "}")
	block := rest[:end]

	required := []string{"WidgetType", "WidgetConfig", "CardType", "ExpandParam"}
	for _, f := range required {
		if !strings.Contains(block, f) {
			t.Errorf("卡片 updateField 缺少 %s —— 该字段将无法保存", f)
		}
	}
}

// TestCardItemUpdateRequiresOwnership 卡片更新同样要限制归属
func TestCardItemUpdateRequiresOwnership(t *testing.T) {
	src := readSource(t, "itemIcon.go")
	if !strings.Contains(src, `"id = ? AND user_id = ?"`) {
		t.Error("卡片更新缺少 user_id 条件 —— 存在越权改他人卡片的风险")
	}
}
