package wgt

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Stat 单项指标
type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Tone  string `json:"tone,omitempty"` // good / warn / bad / muted
}

// Result 小组件输出
type Result struct {
	OK       bool       `json:"ok"`
	Error    string     `json:"error,omitempty"`
	Text     string     `json:"text,omitempty"`
	Stats    []Stat     `json:"stats,omitempty"`
	Detail   [][]string `json:"detail,omitempty"`
	Progress *Progress  `json:"progress,omitempty"`
}

// Progress 进度条
type Progress struct {
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

func errResult(format string, a ...interface{}) *Result {
	return &Result{OK: false, Error: fmt.Sprintf(format, a...)}
}

func newClient(insecure bool, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecure},
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// FormatBytes 人类可读字节
func FormatBytes(v uint64) string {
	const u = 1024
	if v < u {
		return fmt.Sprintf("%dB", v)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	d, e := float64(u), 0
	for n := v / u; n >= u && e < len(units)-1; n /= u {
		d *= u
		e++
	}
	f := float64(v) / d
	if f < 10 {
		return fmt.Sprintf("%.1f%s", f, units[e])
	}
	return fmt.Sprintf("%.0f%s", f, units[e])
}

// FormatSpeed 速度
func FormatSpeed(v uint64) string {
	return FormatBytes(v) + "/s"
}

// FormatDur 时长
func FormatDur(sec int64) string {
	if sec <= 0 {
		return "—"
	}
	d := sec / 86400
	h := (sec % 86400) / 3600
	m := (sec % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func trimBase(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func tone(cond bool, t string) string {
	if cond {
		return t
	}
	return ""
}
