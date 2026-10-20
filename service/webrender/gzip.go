package webrender

import (
	"compress/gzip"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipPool = sync.Pool{New: func() interface{} { return gzip.NewWriter(nil) }}

var compressible = []string{
	"text/html", "text/css", "text/plain", "text/xml", "text/javascript",
	"application/javascript", "application/json", "application/xml",
	"image/svg+xml",
}

func shouldCompress(ct string) bool {
	ct = strings.ToLower(ct)
	for _, c := range compressible {
		if strings.Contains(ct, c) {
			return true
		}
	}
	return false
}

type gzipWriter struct {
	gin.ResponseWriter
	gz          *gzip.Writer
	compress    bool
	wroteHeader bool
	status      int
}

// WriteHeader 记录状态码。Gin 的 WriteHeader 只存不写，真正写出在 WriteHeaderNow。
func (w *gzipWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// WriteHeaderNow 在真正写出前决定是否压缩。
// 3xx/4xx/5xx 一律不压缩：重定向响应体极小，包装 Writer 会破坏状态码语义。
func (w *gzipWriter) WriteHeaderNow() {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	if w.status >= 300 || w.status < 200 {
		w.compress = false
	} else if shouldCompress(w.Header().Get("Content-Type")) {
		w.compress = true
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.Header().Add("Vary", "Accept-Encoding")
		w.gz = gzipPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		if w.status == 0 {
			w.status = 200
		}
		w.WriteHeaderNow()
	}
	if w.compress && w.gz != nil {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipWriter) close() {
	if w.gz != nil {
		_ = w.gz.Close()
		gzipPool.Put(w.gz)
		w.gz = nil
	}
}

// Gzip 压缩中间件
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			c.Next()
			return
		}
		gw := &gzipWriter{ResponseWriter: c.Writer}
		c.Writer = gw
		defer gw.close()
		c.Next()
		gw.close()
	}
}

// StaticCache 给静态资源加长缓存头
func StaticCache(maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			c.Header("Cache-Control", "public, max-age="+itoa(maxAge))
		}
		c.Next()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
