package pages

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// contextWithTimeout 带超时的后台 context
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// randomToken 生成 n 字节的随机十六进制串（长度为 2n）
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 退化：用时间戳填充，保证不 panic
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (i % 8 * 8))
		}
	}
	return hex.EncodeToString(b)
}
