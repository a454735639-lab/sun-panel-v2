package pages

import (
	"net/http"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/lib/cmn"
	"sun-panel/models"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// Panel 首页
func Panel(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	groups, err := loadGroups(c, user.ID)
	if err != nil {
		// 注意：模板读的是 .Error（原先传的是 .Message，导致错误文字显示不出来）
		webrender.Render(c, http.StatusInternalServerError, "error.html", gin.H{
			"Title":     "出错了",
			"SiteTitle": siteTitle(),
			"Theme":     themeMode(c),
			"Error":     "加载面板数据失败。",
			"Hints": []string{
				"确认数据库文件可读写（默认在 data/database/ 下）。",
				"若刚升级过版本，检查数据目录挂载是否正确。",
				"查看容器日志中同时刻的报错以定位具体原因。",
			},
			"Detail": err.Error(),
		})
		return
	}
	attachContainerStates(groups)

	webrender.Render(c, http.StatusOK, "panel.html", gin.H{
		"Title":           siteTitle(),
		"SiteTitle":       siteTitle(),
		"Theme":           themeMode(c),
		"BodyClass":       "panel-page",
		"LoggedIn":        true,
		"UserName":        user.Name,
		"Groups":          groups,
		"Panel":           loadPanelConfig(user.ID),
		"NetMode":         netMode(c),
		"DockerAvailable": global.DockerClient != nil,
	})
}

// LoginPage 登录页
func LoginPage(c *gin.Context) {
	if _, ok := currentUser(c); ok {
		c.Redirect(http.StatusFound, "/")
		return
	}
	webrender.Render(c, http.StatusOK, "login.html", gin.H{
		"Title":     "登录",
		"SiteTitle": siteTitle(),
		"Theme":     themeMode(c),
		"BodyClass": "login-page",
	})
}

// LoginSubmit 处理登录表单
func LoginSubmit(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	clientIP := getClientIP(c)

	if allowed, waitSec := checkLoginAllowed(clientIP, username); !allowed {
		webrender.Render(c, http.StatusTooManyRequests, "login.html", gin.H{
			"Title":     "登录",
			"SiteTitle": siteTitle(),
			"Theme":     themeMode(c),
			"Error":     "尝试次数过多，请 " + itoaSec(waitSec) + " 分钟后再试",
		})
		return
	}

	if username == "" || password == "" {
		recordLoginFailure(clientIP, username)
		webrender.Render(c, http.StatusOK, "login.html", gin.H{
			"Title":     "登录",
			"SiteTitle": siteTitle(),
			"Theme":     themeMode(c),
			"Error":     "请输入账号和密码",
		})
		return
	}

	mUser := models.User{}
	u, err := mUser.GetUserInfoByUsernameAndPassword(username, cmn.PasswordEncryption(password))
	if err != nil || u.ID == 0 {
		recordLoginFailure(clientIP, username)
		webrender.Render(c, http.StatusOK, "login.html", gin.H{
			"Title":     "登录",
			"SiteTitle": siteTitle(),
			"Theme":     themeMode(c),
			"Error":     "账号或密码错误",
		})
		return
	}
	if u.Status != 1 {
		webrender.Render(c, http.StatusOK, "login.html", gin.H{
			"Title":     "登录",
			"SiteTitle": siteTitle(),
			"Theme":     themeMode(c),
			"Error":     "账号已被禁用",
		})
		return
	}

	resetLoginFailures(clientIP, username)

	// 老库可能没有 token，登录时补生成
	if strings.TrimSpace(u.Token) == "" {
		u.Token = randomToken(16)
		if err := global.Db.Model(&models.User{}).Where("id = ?", u.ID).
			Update("token", u.Token).Error; err != nil {
			c.String(http.StatusInternalServerError, "生成会话失败")
			return
		}
	}

	if global.UserToken != nil {
		global.UserToken.Set(u.Token, u, 24*time.Hour)
	}
	c.SetCookie(SessionCookieName, u.Token, 86400*7, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// Logout 退出登录
func Logout(c *gin.Context) {
	if v, err := c.Cookie(SessionCookieName); err == nil && v != "" {
		if global.UserToken != nil {
			global.UserToken.Delete(v)
		}
	}
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// Healthz 健康检查
func Healthz(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

func itoaSec(sec int) string {
	if sec <= 0 {
		return "1"
	}
	return itoaInt(sec/60 + 1)
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
