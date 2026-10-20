package models

import (
	"strings"
)

// MatchUrlPattern 判断 url 是否匹配 pattern。
//
// 支持的语法（对齐原版 v2 的"支持通配符和端口"）：
//   - `*`   匹配任意字符（含空串）
//   - 其余字符按字面量匹配，**忽略大小写**（域名不区分大小写）
//
// 匹配目标**同时尝试三种视图**，因为用户写规则时习惯不同：
//
//	完整 URL        "https://a.example.com/path"
//	主机[:端口]      "a.example.com:8080"
//	纯主机名(去端口)  "a.example.com"
//
// 举例：
//
//	pattern="*.example.com"   配 "https://a.example.com/path" → 命中
//	pattern="example.com:8080" 配 "http://example.com:8080/x"  → 命中
//	pattern="nas.local"       配 "nas.local:5000"             → 命中（端口可省）
//
// 设计取舍：
//   - 不做正则（原版明确"暂不支持正则表达式"），避免用户误写导致灾难性回溯
//   - 通配符转换用逐字符扫描，不用 regexp，既快又可预测
func MatchUrlPattern(pattern, url string) bool {
	pattern = strings.TrimSpace(pattern)
	url = strings.TrimSpace(url)
	if pattern == "" || url == "" {
		return false
	}
	if pattern == "*" {
		return true
	}

	host := extractHost(url)
	candidates := []string{url, host}
	// 主机去端口后再试一次：让 "nas.local" 能命中 "nas.local:5000"。
	// 只在 pattern 未显式包含端口时才加这个视图 —— 否则
	// "example.com:8080" 会错误地命中 "example.com:9090" 的无端口视图。
	if !strings.Contains(pattern, ":") {
		if h, _, ok := strings.Cut(host, ":"); ok && h != "" {
			candidates = append(candidates, h)
		}
	}

	pl := strings.ToLower(pattern)
	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		if wildcardMatch(pl, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

// extractHost 从 URL 中取 主机[:端口]。
// 不依赖 net/url，因为用户填的可能是 "192.168.1.1:8080" 这种非完整 URL。
func extractHost(raw string) string {
	s := raw
	// 去掉协议
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// 去掉用户信息 user:pass@
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	// 去掉路径/查询/片段
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// wildcardMatch 支持 `*` 通配符的字符串匹配（大小写需调用方已统一）。
//
// 采用经典的线性回溯算法（贪心 + 回退），复杂度 O(n*m) 最坏，
// 但无回溯爆炸风险（对比正则），适合处理用户填写的规则。
func wildcardMatch(pattern, s string) bool {
	pi, si := 0, 0
	star := -1     // 最近一个 * 在 pattern 中的位置
	starMatch := 0 // 该 * 当时匹配到的 s 位置

	for si < len(s) {
		switch {
		case pi < len(pattern) && pattern[pi] == '*':
			// 记录回退点
			star = pi
			starMatch = si
			pi++
		case pi < len(pattern) && pattern[pi] == s[si]:
			pi++
			si++
		case star >= 0:
			// 失配，让上一个 * 多吃一个字符再试
			starMatch++
			si = starMatch
			pi = star + 1
		default:
			return false
		}
	}
	// 收尾：pattern 剩余的必须全是 *
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// PickUrlByRules 按规则逐条匹配，返回命中的 Target。
//
// rules 需按 Sort 升序预先排好（调用方负责）。
// 返回 (目标地址, 是否命中)。未命中时调用方应回落到内置 IP 判断。
//
// 为什么 target 直接返回而不做变量替换：
// 保持语义简单可预测。用户要"内网地址"就填内网地址，不做隐式替换。
func PickUrlByRules(rules []UrlRule, url string) (string, bool) {
	for i := range rules {
		r := rules[i]
		if r.Enabled != INT_TURE {
			continue
		}
		target := strings.TrimSpace(r.Target)
		if target == "" {
			continue
		}
		if MatchUrlPattern(r.Pattern, url) {
			return target, true
		}
	}
	return "", false
}
