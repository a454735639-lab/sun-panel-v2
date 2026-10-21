package siteFavicon

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sun-panel/lib/cmn"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func IsHTTPURL(url string) bool {
	httpPattern := `^(http://|https://|//)`
	match, err := regexp.MatchString(httpPattern, url)
	if err != nil {
		return false
	}
	return match
}

// FallbackFaviconURLs 返回一串"按顺序尝试"的第三方 favicon 服务地址。
//
// 为什么需要多个而不是只用 Google：`https://www.google.com/s2/favicons`
// 在中国大陆网络下不可达（实测连接超时 8s），而这恰恰是本项目的主要使用场景。
// 原先失败后一路回退到 Google，结果就是"图标加载不出来"——
// 且因为回退本身也失败，界面上看不出是哪一步挂了。
//
// 这里优先用大陆可达的服务，Google 放在最后（海外部署时仍能兜底）。
//
// 注意：调用方**不能**对这些 URL 去除查询参数 ——
// 域名是通过 ?domain= 传的，剥掉参数就变成请求服务首页，拿不到目标图标。
func FallbackFaviconURLs(domain string) []string {
	d := strings.TrimSpace(domain)
	if d == "" {
		return nil
	}
	// 去掉端口，favicon 服务一般不接受 host:port 形式
	if i := strings.LastIndex(d, ":"); i > 0 && !strings.Contains(d[i:], "]") {
		d = d[:i]
	}
	return []string{
		fmt.Sprintf("https://favicon.im/%s?larger=true", d),
		fmt.Sprintf("https://favicon.zhusl.com/ico?url=%s", d),
		fmt.Sprintf("https://toolb.cn/favicon/%s", d),
		fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", d),
	}
}

// StripQuery 去掉 URL 的查询串，但保留第三方 favicon 服务的参数。
//
// 原实现无条件剥掉全部 get 参数，本意是"有些图标服务器不支持带参请求"，
// 但副作用是把 Google/其他服务的 domain 参数也剥了 ——
// 变成请求 `https://www.google.com/s2/favicons`（无参数），
// 拿到的是 Google 自己的图标而非目标网站，等于回退失效。
func StripQuery(rawURL string) string {
	p, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// 这些服务的查询串是语义的一部分，必须保留
	if strings.Contains(p.Host, "google.com") ||
		strings.Contains(p.Host, "favicon.im") ||
		strings.Contains(p.Host, "zhusl.com") ||
		strings.Contains(p.Host, "toolb.cn") {
		return rawURL
	}
	if p.RawQuery == "" {
		return rawURL
	}
	p.RawQuery = ""
	return p.String()
}

func GetOneFaviconURL(urlStr string) (string, error) {
	iconURLs, err := getFaviconURL(urlStr)
	if err != nil {
		return "", err
	}

	for _, v := range iconURLs {
		// 标准的路径地址
		if IsHTTPURL(v) {
			return v, nil
		} else {
			urlInfo, _ := url.Parse(urlStr)
			fullUrl := urlInfo.Scheme + "://" + urlInfo.Host + "/" + strings.TrimPrefix(v, "/")
			return fullUrl, nil
		}
	}
	return "", fmt.Errorf("not found ico")
}

// 获取远程文件的大小
func GetRemoteFileSize(url string) (int64, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// 检查HTTP响应状态
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP request failed, status code: %d", resp.StatusCode)
	}

	// 获取Content-Length字段，即文件大小
	size := resp.ContentLength
	return size, nil
}

// 下载图片
func DownloadImage(url, savePath string, maxSize int64) (*os.File, error) {
	// 获取远程文件大小
	fileSize, err := GetRemoteFileSize(url)
	if err != nil {
		return nil, err
	}

	// 判断文件大小是否在阈值内
	if fileSize > maxSize {
		return nil, fmt.Errorf("文件太大，不下载。大小：%d字节", fileSize)
	}

	// 发送HTTP GET请求获取图片数据
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 检查HTTP响应状态
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed, status code: %d", response.StatusCode)
	}

	urlFileName := path.Base(url)
	fileExt := path.Ext(url)
	fileName := cmn.Md5(fmt.Sprintf("%s%s", urlFileName, time.Now().String())) + fileExt

	destination := savePath + "/" + fileName

	// 创建本地文件用于保存图片
	file, err := os.Create(destination)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 将图片数据写入本地文件
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func GetOneFaviconURLAndUpload(urlStr string) (string, bool) {
	//www.iqiyipic.com/pcwimg/128-128-logo.png
	iconURLs, err := getFaviconURL(urlStr)
	if err != nil {
		return "", false
	}

	for _, v := range iconURLs {
		// 标准的路径地址
		if IsHTTPURL(v) {
			return v, true
		} else {
			urlInfo, _ := url.Parse(urlStr)
			fullUrl := urlInfo.Scheme + "://" + urlInfo.Host + "/" + strings.TrimPrefix(v, "/")
			return fullUrl, true
		}
	}
	return "", false
}

func getFaviconURL(url string) ([]string, error) {
	var icons []string
	icons = make([]string, 0)
	client := &http.Client{
		Timeout: 5 * time.Second, // 设置超时时间
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return icons, err
	}

	// 设置User-Agent头字段，模拟现代浏览器请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")

	resp, err := client.Do(req)
	if err != nil {
		return icons, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return icons, errors.New("HTTP request failed with status code " + strconv.Itoa(resp.StatusCode))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return icons, err
	}

	// 查找所有link标签，支持更多rel属性值
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		rel, _ := s.Attr("rel")
		href, _ := s.Attr("href")

		// 支持多种rel属性值，包括icon, shortcut icon, apple-touch-icon等
		relLower := strings.ToLower(rel)
		if (strings.Contains(relLower, "icon") || strings.Contains(relLower, "shortcut")) && href != "" {
			icons = append(icons, href)
		}
	})

	// 如果没有找到link标签，尝试添加默认的favicon.ico路径
	if len(icons) == 0 {
		icons = append(icons, "/favicon.ico")
	}

	return icons, nil
}
