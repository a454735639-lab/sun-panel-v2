package models

import "testing"

// TestMatchUrlPattern 覆盖通配符与端口匹配。
// 这是地址规则的核心，错一个字符用户就点不开站点，必须测足。
func TestMatchUrlPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		url     string
		want    bool
	}{
		// ---- 通配符 ----
		{"星号-匹配任意", "*", "https://anything.com/x", true},
		{"前缀通配-子域名命中", "*.example.com", "https://a.example.com/path", true},
		{"前缀通配-多层子域", "*.example.com", "https://a.b.example.com/path", true},
		{"前缀通配-裸域不命中", "*.example.com", "https://example.com/path", false},
		{"后缀通配", "192.168.*", "http://192.168.1.100:8080/", true},
		{"中间通配", "192.168.*.1", "192.168.5.1", true},
		{"多星号", "*example*", "https://my-example-site.com", true},

		// ---- 端口 ----
		{"带端口命中", "example.com:8080", "http://example.com:8080/api", true},
		{"带端口-端口不符", "example.com:8080", "http://example.com:9090/api", false},
		{"通配符加端口", "*.example.com:8080", "http://a.example.com:8080/x", true},

		// ---- 大小写（域名不区分大小写）----
		{"大小写不敏感-pattern大写", "EXAMPLE.COM", "https://example.com/x", true},
		{"大小写不敏感-url大写", "example.com", "https://EXAMPLE.COM/x", true},
		{"大小写不敏感-通配", "*.Example.COM", "https://a.EXAMPLE.com/x", true},

		// ---- 纯主机名（无协议）----
		{"无协议纯主机", "nas.local", "nas.local:5000", true},
		{"无协议IP带端口", "10.0.0.5:8080", "10.0.0.5:8080", true},

		// ---- 应拒绝 ----
		{"空pattern", "", "https://example.com", false},
		{"空url", "example.com", "", false},
		{"全空", "", "", false},
		{"不匹配", "example.com", "https://other.com", false},

		// ---- 边界：不应被误当通配 ----
		{"点号非通配", "a.example.com", "Xa.example.com", false},
		{"不含星号须精确", "example.com", "example.com.cn", false},

		// ---- 精确匹配 ----
		{"精确完整URL", "https://example.com/path", "https://example.com/path", true},
		{"精确主机", "example.com", "https://example.com/whatever", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchUrlPattern(tt.pattern, tt.url); got != tt.want {
				t.Errorf("MatchUrlPattern(%q, %q) = %v, 期望 %v",
					tt.pattern, tt.url, got, tt.want)
			}
		})
	}
}

// TestExtractHost 主机提取是匹配的前提，独立测。
func TestExtractHost(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://example.com/path?q=1", "example.com"},
		{"http://example.com:8080/api", "example.com:8080"},
		{"example.com", "example.com"},
		{"example.com:9090", "example.com:9090"},
		{"https://user:pass@example.com/x", "example.com"},
		{"http://192.168.1.1:5000/", "192.168.1.1:5000"},
		{"https://example.com#frag", "example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := extractHost(tt.in); got != tt.want {
				t.Errorf("extractHost(%q) = %q, 期望 %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestPickUrlByRules 规则按序匹配、跳过禁用项。
func TestPickUrlByRules(t *testing.T) {
	rules := []UrlRule{
		{Pattern: "*.inner.local", Target: "http://10.0.0.1:8080", Enabled: INT_TURE, Sort: 1},
		{Pattern: "*.example.com", Target: "https://public.example.com", Enabled: INT_TURE, Sort: 2},
		{Pattern: "*.disabled.com", Target: "http://never", Enabled: INT_FALSE, Sort: 3},
		{Pattern: "*", Target: "http://fallback", Enabled: INT_TURE, Sort: 4},
	}

	tests := []struct {
		name    string
		url     string
		wantURL string
		wantHit bool
	}{
		{"命中第一条", "http://svc.inner.local/x", "http://10.0.0.1:8080", true},
		{"命中第二条", "https://a.example.com", "https://public.example.com", true},
		{"禁用规则被跳过", "http://a.disabled.com", "http://fallback", true},
		{"全通配兜底", "http://random.io", "http://fallback", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hit := PickUrlByRules(rules, tt.url)
			if hit != tt.wantHit {
				t.Errorf("PickUrlByRules(%q) hit = %v, 期望 %v", tt.url, hit, tt.wantHit)
			}
			if got != tt.wantURL {
				t.Errorf("PickUrlByRules(%q) = %q, 期望 %q", tt.url, got, tt.wantURL)
			}
		})
	}
}

// TestPickUrlByRules_Empty 无规则/空 target 必须不命中，让调用方回落内置判断。
// 这条是兼容性契约：老用户没有规则时行为必须与改造前一致。
func TestPickUrlByRules_Empty(t *testing.T) {
	tests := []struct {
		name  string
		rules []UrlRule
		url   string
	}{
		{"无规则", nil, "https://example.com"},
		{"空规则列表", []UrlRule{}, "https://example.com"},
		{"target为空", []UrlRule{{Pattern: "*", Target: "", Enabled: INT_TURE}}, "https://x.com"},
		{"target全空格", []UrlRule{{Pattern: "*", Target: "   ", Enabled: INT_TURE}}, "https://x.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, hit := PickUrlByRules(tt.rules, tt.url); hit {
				t.Errorf("应不命中以回落内置判断，实际命中")
			}
		})
	}
}
