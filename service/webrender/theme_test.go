package webrender

import (
	"strings"
	"testing"
)

// TestNormalizeColor_Valid 合法颜色必须被正确规范化。
func TestNormalizeColor_Valid(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// hex
		{"#abc", "#abc"},
		{"#ABC", "#abc"},          // 转小写
		{"#4ec9a7", "#4ec9a7"},
		{"#4EC9A7", "#4ec9a7"},
		{"#4ec9a7ff", "#4ec9a7ff"},
		{"  #4ec9a7  ", "#4ec9a7"}, // 去空格
		// rgb
		{"rgb(255,255,255)", "rgb(255,255,255)"},
		{"rgb(0, 0, 0)", "rgb(0,0,0)"},
		// rgba
		{"rgba(0,0,0,0.5)", "rgba(0,0,0,0.5)"},
		{"RGBA(28, 32, 38, 0.42)", "rgba(28,32,38,0.42)"},
		// alpha 0-255 归一化（输出最多两位小数，去尾随零）
		{"rgba(0,0,0,128)", "rgba(0,0,0,0.5)"},
		{"rgba(0,0,0,255)", "rgba(0,0,0,1)"},
		{"rgba(0,0,0,13)", "rgba(0,0,0,0.05)"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeColor(tt.in); got != tt.want {
				t.Errorf("NormalizeColor(%q) = %q, 期望 %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestNormalizeColor_Reject 非法/危险输入必须返回空串。
// 这是防 CSS 注入的核心闸门。
func TestNormalizeColor_Reject(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"空串", ""},
		{"仅空格", "   "},
		// ---- CSS 注入尝试 ----
		{"注入-闭合花括号", "red; } body { display:none } .x{"},
		{"注入-带分号", "#fff; background:url(evil)"},
		{"注入-带尖括号", "</style><script>alert(1)</script>"},
		{"注入-带引号", `"#fff"`},
		{"注入-注释起手", "/* x */ #fff"},
		// ---- 白名单外格式 ----
		{"命名颜色", "red"},
		{"命名颜色-透明", "transparent"},
		{"hsl", "hsl(120,50%,50%)"},
		{"var引用", "var(--accent)"},
		{"url", "url(http://evil/x.png)"},
		{"expression", "expression(alert(1))"},
		// ---- 结构错误 ----
		{"hex长度错-2位", "#ab"},
		{"hex长度错-4位", "#abcd"},
		{"hex长度错-5位", "#abcde"},
		{"hex含非hex字符", "#gggggg"},
		{"rgb分量越界-高", "rgb(256,0,0)"},
		{"rgb分量越界-负", "rgb(-1,0,0)"},
		{"rgb分量非数字", "rgb(a,b,c)"},
		{"rgb分量缺失", "rgb(1,2)"},
		{"rgb分量过多", "rgb(1,2,3,4)"},
		{"rgb未闭合", "rgb(1,2,3"},
		{"rgba-alpha越界", "rgba(0,0,0,1.5)"},
		{"rgba-alpha负", "rgba(0,0,0,-0.1)"},
		// ---- 超长 ----
		{"超长输入", "#" + strings.Repeat("a", 200)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeColor(tt.in); got != "" {
				t.Errorf("NormalizeColor(%q) = %q，应拒绝但被接受", tt.in, got)
			}
		})
	}
}

// TestThemeStyle 生成 CSS 时必须过滤非法值、拒绝非法变量名。
func TestThemeStyle(t *testing.T) {
	t.Run("全部合法", func(t *testing.T) {
		got := ThemeStyle(map[string]string{
			"--accent": "#4ec9a7",
		})
		if !strings.Contains(got, "--accent:#4ec9a7") {
			t.Errorf("应包含 --accent:#4ec9a7，实际 %q", got)
		}
		if !strings.HasPrefix(got, ":root{") || !strings.HasSuffix(got, "}") {
			t.Errorf("CSS 结构不对: %q", got)
		}
	})

	t.Run("非法值被过滤", func(t *testing.T) {
		got := ThemeStyle(map[string]string{
			"--accent": "#4ec9a7",
			"--evil":   "red; } body { display:none } .x{",
		})
		if strings.Contains(got, "display:none") {
			t.Fatalf("注入内容泄漏进 CSS: %q", got)
		}
		if !strings.Contains(got, "--accent") {
			t.Errorf("合法项应保留: %q", got)
		}
		if strings.Contains(got, "--evil") {
			t.Errorf("非法项应被丢弃: %q", got)
		}
	})

	t.Run("非法变量名被过滤", func(t *testing.T) {
		got := ThemeStyle(map[string]string{
			"--ok-name":     "#fff",
			"bad name":      "#fff",
			"--UPPER":       "#fff",
			"-single":       "#fff",
			"--with;semi":   "#fff",
		})
		if !strings.Contains(got, "--ok-name") {
			t.Errorf("合法变量名应保留: %q", got)
		}
		for _, bad := range []string{"bad name", "--UPPER", "-single", "semi"} {
			if strings.Contains(got, bad) {
				t.Errorf("非法变量名 %q 应被丢弃: %q", bad, got)
			}
		}
	})

	t.Run("全非法时返回空串", func(t *testing.T) {
		got := ThemeStyle(map[string]string{
			"--a": "red; } x{",
			"--b": "notacolor",
		})
		if got != "" {
			t.Errorf("全非法应返回空串（不注入任何 style），实际 %q", got)
		}
	})

	t.Run("空输入返回空串", func(t *testing.T) {
		if got := ThemeStyle(nil); got != "" {
			t.Errorf("nil 应返回空串，实际 %q", got)
		}
		if got := ThemeStyle(map[string]string{}); got != "" {
			t.Errorf("空 map 应返回空串，实际 %q", got)
		}
	})
}

// TestIsSafeVarName 变量名白名单。
func TestIsSafeVarName(t *testing.T) {
	ok := []string{"--accent", "--accent-2", "--a1", "--x-y-z"}
	for _, n := range ok {
		if !isSafeVarName(n) {
			t.Errorf("%q 应被允许", n)
		}
	}
	bad := []string{"", "-", "--", "accent", "--Accent", "--a b", "--a;b", "--a{}", "--" + strings.Repeat("x", 60)}
	for _, n := range bad {
		if isSafeVarName(n) {
			t.Errorf("%q 应被拒绝", n)
		}
	}
}