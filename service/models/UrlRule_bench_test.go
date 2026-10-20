package models

import (
	"strings"
	"testing"
)

// 说明：pickUrl 的 N+1 问题发生在 pages 包（依赖 gin.Context 与数据库），
// 这里只对**纯匹配路径**做基准，量化单次匹配的开销，
// 以便判断"每卡片一次匹配"本身是否构成瓶颈。
//
// 结论参考：单次匹配在微秒级，不是瓶颈；真正的瓶颈是它外面套的数据库查询。
// 因此修复方向是"规则只查一次"，而不是"优化匹配算法"。

func benchRules(n int) []UrlRule {
	rules := make([]UrlRule, 0, n)
	for i := 0; i < n; i++ {
		rules = append(rules, UrlRule{
			Pattern: "*host" + strings.Repeat("x", i%8) + ".example.com",
			Target:  "http://10.0.0.1:8080",
			Enabled: INT_TURE,
			Sort:    i,
		})
	}
	// 最后放一个必定命中的兜底，模拟最坏情况（要走完全部规则）
	rules = append(rules, UrlRule{Pattern: "*", Target: "http://fallback", Enabled: INT_TURE, Sort: n})
	return rules
}

func BenchmarkPickUrlByRules(b *testing.B) {
	for _, n := range []int{1, 10, 50} {
		rules := benchRules(n)
		url := "http://some-host.example.com/app"
		b.Run(strings.Repeat("_", 1)+itoa(n)+"条规则", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = PickUrlByRules(rules, url)
			}
		})
	}
}

func BenchmarkMatchUrlPattern(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = MatchUrlPattern("*.example.com:8080", "http://a.b.example.com:8080/path")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}