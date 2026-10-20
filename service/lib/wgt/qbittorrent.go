package wgt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// QBittorrent qB 小组件
func QBittorrent(cfg map[string]string) *Result {
	base := trimBase(cfg["url"])
	if base == "" {
		return errResult("未配置 qBittorrent 地址")
	}
	apiKey := strings.TrimSpace(cfg["apikey"])
	jar, _ := cookiejar.New(nil)
	c := newClient(true, 8*time.Second)
	c.Jar = jar

	auth := func(req *http.Request) {
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	// 无 API Key 时走账号密码登录
	if apiKey == "" {
		form := url.Values{}
		form.Set("username", cfg["username"])
		form.Set("password", cfg["password"])
		req, _ := http.NewRequest(http.MethodPost, base+"/api/v2/auth/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", base)
		resp, err := c.Do(req)
		if err != nil {
			return errResult("连接失败：%v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), "Ok.") {
			return errResult("登录失败：%s", strings.TrimSpace(string(body)))
		}
	}

	get := func(path string) ([]byte, error) {
		req, err := http.NewRequest(http.MethodGet, base+path, nil)
		if err != nil {
			return nil, err
		}
		auth(req)
		req.Header.Set("Referer", base)
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return b, nil
	}

	b, err := get("/api/v2/transfer/info")
	if err != nil {
		return errResult("获取传输信息失败：%v", err)
	}
	var transfer struct {
		DlInfoSpeed int64 `json:"dl_info_speed"`
		UpInfoSpeed int64 `json:"up_info_speed"`
	}
	_ = json.Unmarshal(b, &transfer)

	tb, err := get("/api/v2/torrents/info")
	if err != nil {
		return errResult("获取种子列表失败：%v", err)
	}
	var torrents []struct {
		State    string  `json:"state"`
		Progress float64 `json:"progress"`
		ETA      int64   `json:"eta"`
		Size     int64   `json:"size"`
	}
	_ = json.Unmarshal(tb, &torrents)

	dlStates := map[string]bool{
		"downloading": true, "stalledDL": true, "metaDL": true, "forcedDL": true,
		"allocating": true, "checkingDL": true, "queuedDL": true,
	}
	upStates := map[string]bool{
		"uploading": true, "stalledUP": true, "forcedUP": true,
		"checkingUP": true, "queuedUP": true,
	}
	downloading, seeding, completed := 0, 0, 0
	var minETA int64 = -1
	for _, t := range torrents {
		if dlStates[t.State] {
			downloading++
			if t.ETA > 0 && (minETA < 0 || t.ETA < minETA) {
				minETA = t.ETA
			}
		}
		if upStates[t.State] {
			seeding++
		}
		if t.Progress >= 0.999 {
			completed++
		}
	}

	res := &Result{
		OK:   true,
		Text: "↓" + FormatSpeed(uint64(max64(transfer.DlInfoSpeed, 0))) + " ↑" + FormatSpeed(uint64(max64(transfer.UpInfoSpeed, 0))),
		Stats: []Stat{
			{Label: "下载", Value: itoa(downloading), Tone: tone(downloading > 0, "warn")},
			{Label: "做种", Value: itoa(seeding)},
		},
		Detail: [][]string{
			{"下载速度", FormatSpeed(uint64(max64(transfer.DlInfoSpeed, 0)))},
			{"上传速度", FormatSpeed(uint64(max64(transfer.UpInfoSpeed, 0)))},
			{"下载中", itoa(downloading)},
			{"做种中", itoa(seeding)},
			{"已完成", itoa(completed)},
			{"任务总数", itoa(len(torrents))},
		},
	}
	if minETA > 0 {
		res.Detail = append(res.Detail, []string{"最快完成", FormatDur(minETA)})
	}
	return res
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
