package webrender

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

var (
	once     sync.Once
	tmpl     *template.Template
	tmplErr  error
	assetVer string
	avOnce   sync.Once
)

// FuncMap 模板辅助函数
var FuncMap = template.FuncMap{
	"formatBytes":  FormatBytes,
	"AssetVersion": AssetVersion,
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04")
	},
	"lower": strings.ToLower,
	"default": func(def, v interface{}) interface{} {
		if v == nil {
			return def
		}
		switch x := v.(type) {
		case string:
			if x == "" {
				return def
			}
		case int:
			if x == 0 {
				return def
			}
		}
		return v
	},
	"dict": func(kv ...interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for i := 0; i+1 < len(kv); i += 2 {
			k, ok := kv[i].(string)
			if !ok {
				continue
			}
			m[k] = kv[i+1]
		}
		return m
	},
	// textOnBg 按背景色亮度返回黑或白文字色。
	// 复刻原版 AppIcon 的 calculateLuminance：亮度 > 0.5 用黑，否则用白。
	"textOnBg": TextOnBg,
	"bgSrc": func(p string) string {
		if p == "" || p == "defaultBackground.jpg" {
			return "/static/defaultBackground.jpg"
		}
		return p
	},
}

// DefaultIconBg 原版图标/卡片的默认背景色
const DefaultIconBg = "#2a2a2a6b"

// TextOnBg 按背景色亮度选择可读的文字颜色，返回 "#000000" 或 "#ffffff"。
// 支持 #rgb / #rrggbb / #rrggbbaa / rgba(...) / rgb(...)；解析失败按深色处理。
func TextOnBg(bg string) string {
	r, g, b, ok := parseColor(bg)
	if !ok {
		return "#ffffff"
	}
	lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255
	if lum > 0.5 {
		return "#000000"
	}
	return "#ffffff"
}

// parseColor 解析颜色为 0-255 的 r,g,b
func parseColor(s string) (int, int, int, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, 0, 0, false
	}
	if strings.HasPrefix(s, "rgb") {
		i := strings.Index(s, "(")
		j := strings.Index(s, ")")
		if i < 0 || j < i {
			return 0, 0, 0, false
		}
		parts := strings.Split(s[i+1:j], ",")
		if len(parts) < 3 {
			return 0, 0, 0, false
		}
		var v [3]int
		for k := 0; k < 3; k++ {
			n, err := strconv.Atoi(strings.TrimSpace(parts[k]))
			if err != nil {
				return 0, 0, 0, false
			}
			v[k] = n
		}
		return v[0], v[1], v[2], true
	}
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		switch len(h) {
		case 3: // #rgb
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		case 4: // #rgba
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		case 6, 8: // #rrggbb / #rrggbbaa（忽略 alpha）
		default:
			return 0, 0, 0, false
		}
		if len(h) < 6 {
			return 0, 0, 0, false
		}
		n, err := strconv.ParseUint(h[:6], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return int(n>>16) & 0xff, int(n>>8) & 0xff, int(n) & 0xff, true
	}
	return 0, 0, 0, false
}

// FormatBytes 人类可读的字节数
func FormatBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%dB", v)
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	f := float64(v) / float64(div)
	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	if f < 10 {
		return fmt.Sprintf("%.1f%s", f, units[exp])
	}
	return fmt.Sprintf("%.0f%s", f, units[exp])
}

// AssetVersion 静态资源内容指纹，用于破缓存
func AssetVersion() string {
	avOnce.Do(func() {
		h := fnv.New32a()
		sub, err := fs.Sub(staticFS, "static")
		if err != nil {
			assetVer = "1"
			return
		}
		for _, name := range []string{"app.css", "app.js"} {
			if b, err := fs.ReadFile(sub, name); err == nil {
				_, _ = h.Write(b)
			}
		}
		assetVer = fmt.Sprintf("%08x", h.Sum32())
	})
	return assetVer
}

// T 返回模板集合。若设置了 SUNPANEL_TEMPLATE_DIR 则优先从磁盘加载（免重编）。
func T() (*template.Template, error) {
	if dir := os.Getenv("SUNPANEL_TEMPLATE_DIR"); dir != "" {
		if t, e := buildFromDir(dir); e == nil {
			return t, nil
		}
	}
	once.Do(func() {
		tmpl, tmplErr = buildFromEmbed()
	})
	return tmpl, tmplErr
}

func buildFromDir(dir string) (*template.Template, error) {
	t := template.New("").Funcs(FuncMap)
	matches, _ := filepath.Glob(filepath.Join(dir, "*.html"))
	if len(matches) == 0 {
		return nil, fmt.Errorf("no templates in %s", dir)
	}
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		if _, err := t.New(filepath.Base(m)).Parse(string(b)); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(m), err)
		}
	}
	return t, nil
}

func buildFromEmbed() (*template.Template, error) {
	t := template.New("").Funcs(FuncMap)
	entries, err := templatesFS.ReadDir("templates")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		b, err := fs.ReadFile(templatesFS, "templates/"+e.Name())
		if err != nil {
			return nil, err
		}
		if _, err := t.New(e.Name()).Parse(string(b)); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return t, nil
}

// Render 渲染整页。data 可为 gin.H / map / 结构体。
func Render(c *gin.Context, status int, name string, data interface{}) {
	t, e := T()
	if e != nil {
		c.String(http.StatusInternalServerError, "template parse error: %v", e)
		return
	}

	h := gin.H{}
	switch v := data.(type) {
	case nil:
	case gin.H:
		for k, val := range v {
			h[k] = val
		}
	case map[string]interface{}:
		for k, val := range v {
			h[k] = val
		}
	}

	var out interface{}
	if len(h) == 0 {
		out = data
	} else {
		h["CurrentPath"] = c.Request.URL.Path
		if _, ok := h["Theme"]; !ok {
			h["Theme"] = "auto"
		}
		out = h
	}

	var buf bytes.Buffer
	if e := t.ExecuteTemplate(&buf, name, out); e != nil {
		c.String(http.StatusInternalServerError, "template exec error: %v", e)
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

// RenderPartial 渲染片段（htmx 局部刷新）
func RenderPartial(c *gin.Context, name string, data interface{}) {
	t, e := T()
	if e != nil {
		c.String(http.StatusInternalServerError, "template parse error: %v", e)
		return
	}
	var buf bytes.Buffer
	if e := t.ExecuteTemplate(&buf, name, data); e != nil {
		c.String(http.StatusInternalServerError, "template exec error: %v", e)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// StaticSub 返回静态资源子文件系统（支持磁盘覆盖）
func StaticSub() (fs.FS, error) {
	if dir := os.Getenv("SUNPANEL_STATIC_DIR"); dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return os.DirFS(dir), nil
		}
	}
	return fs.Sub(staticFS, "static")
}

// RegisterStatic 注册静态资源路由
func RegisterStatic(router *gin.Engine, prefix string) error {
	sub, err := StaticSub()
	if err != nil {
		return err
	}
	handler := http.StripPrefix(prefix, http.FileServer(http.FS(sub)))
	router.GET(prefix+"/*filepath", gin.WrapH(handler))
	router.HEAD(prefix+"/*filepath", gin.WrapH(handler))
	return nil
}
