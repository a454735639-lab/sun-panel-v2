package webrender

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizeColor 校验并规范化用户输入的颜色值。
//
// 为什么必须严格校验：这些值会被拼进 <style> 注入页面，
// 若不校验，用户填 `red; } body { display:none } .x{` 即可注入任意 CSS。
// 因此这里采取**白名单**策略——只接受明确可解析的颜色格式，
// 其他一律返回空串（调用方跳过该变量，退回默认样式）。
//
// 接受的格式：
//
//	#rgb          #abc
//	#rrggbb       #4ec9a7
//	#rrggbbaa     #4ec9a7ff
//	rgb(r,g,b)    分量 0-255
//	rgba(r,g,b,a) a 为 0-1（也接受 0-255 整数，自动归一化）
//
// 返回规范化的字符串（如 "#4EC9A7" → "#4ec9a7"），非法输入返回 ""。
func NormalizeColor(in string) string {
	s := strings.TrimSpace(in)
	if s == "" {
		return ""
	}
	// 长度上限，防止超长字符串攻击
	if len(s) > 64 {
		return ""
	}
	// 一律转小写，便于比较与输出稳定
	s = strings.ToLower(s)

	if strings.HasPrefix(s, "#") {
		return normalizeHex(s)
	}
	if strings.HasPrefix(s, "rgb(") || strings.HasPrefix(s, "rgba(") {
		return normalizeRgbFunc(s)
	}
	// 不接受命名颜色、hsl()、var() 等 —— 白名单外一律拒绝
	return ""
}

// normalizeHex 校验 #rgb / #rrggbb / #rrggbbaa
func normalizeHex(s string) string {
	body := s[1:]
	for _, c := range body {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ""
		}
	}
	switch len(body) {
	case 3, 6, 8:
		return s
	}
	return ""
}

// normalizeRgbFunc 校验 rgb()/rgba() 并规范化输出。
func normalizeRgbFunc(s string) string {
	isRGBA := strings.HasPrefix(s, "rgba(")
	prefix := "rgb("
	if isRGBA {
		prefix = "rgba("
	}
	if !strings.HasSuffix(s, ")") {
		return ""
	}
	inner := s[len(prefix) : len(s)-1]
	parts := strings.Split(inner, ",")

	want := 3
	if isRGBA {
		want = 4
	}
	if len(parts) != want {
		return ""
	}

	out := make([]string, 0, want)
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return ""
		}
		if i < 3 {
			// RGB 分量：0-255 整数
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 || n > 255 {
				return ""
			}
			out = append(out, strconv.Itoa(n))
		} else {
			// alpha：0-1 浮点，或 0-255 整数（归一化）
			if strings.Contains(p, ".") {
				f, err := strconv.ParseFloat(p, 64)
				if err != nil || f < 0 || f > 1 {
					return ""
				}
				out = append(out, strconv.FormatFloat(f, 'f', -1, 64))
			} else {
				n, err := strconv.Atoi(p)
				if err != nil || n < 0 || n > 255 {
					return ""
				}
				if n > 1 {
					// 0-255 → 0-1，并去掉多余的尾随零（1.00 → 1，0.50 → 0.5）
					out = append(out, trimFloat(float64(n)/255.0))
				} else {
					out = append(out, strconv.Itoa(n))
				}
			}
		}
	}

	if isRGBA {
		return fmt.Sprintf("rgba(%s)", strings.Join(out, ","))
	}
	return fmt.Sprintf("rgb(%s)", strings.Join(out, ","))
}

// ThemeStyle 根据主题配置生成注入 <style> 的 CSS 文本。
//
// 只输出**用户显式设置且校验通过**的变量 —— 未设置的不写，
// 让 app.css 里的 :root 默认值继续生效。这样主题功能是"可选增强"，
// 不影响任何未使用该功能的现有用户。
//
// 返回空串表示无需注入。
func ThemeStyle(vars map[string]string) string {
	if len(vars) == 0 {
		return ""
	}
	var sb strings.Builder
	wrote := false
	for name, raw := range vars {
		c := NormalizeColor(raw)
		if c == "" {
			continue
		}
		// 变量名也只允许白名单字符，双重保险
		if !isSafeVarName(name) {
			continue
		}
		if !wrote {
			sb.WriteString(":root{")
			wrote = true
		}
		sb.WriteString(name)
		sb.WriteString(":")
		sb.WriteString(c)
		sb.WriteString(";")
	}
	if !wrote {
		return ""
	}
	sb.WriteString("}")
	return sb.String()
}

// isSafeVarName 变量名白名单：只允许 -- 开头 + 小写字母/数字/连字符，
// 且 -- 后必须至少有一个字符。
func isSafeVarName(n string) bool {
	if !strings.HasPrefix(n, "--") || len(n) <= 2 || len(n) > 40 {
		return false
	}
	for _, c := range n[2:] {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	// 至少含一个字母，避免 "--" / "---" 这类纯连字符
	return strings.ContainsFunc(n[2:], func(c rune) bool {
		return c >= 'a' && c <= 'z'
	})
}

// trimFloat 把 alpha 值格式化为最多两位小数，并去掉多余的尾随零。
//
// 目标：128/255=0.50196... 输出 "0.5"，255/255=1 输出 "1"。
// 直接 FormatFloat(-1) 会输出 17 位小数，既难看又无意义。
func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64) // 先固定两位
	s = strings.TrimRight(s, "0")           // 去尾随零
	return strings.TrimRight(s, ".")        // 若只剩 "0." 则去点
}