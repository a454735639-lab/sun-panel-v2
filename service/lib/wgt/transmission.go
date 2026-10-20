package wgt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Transmission 小组件（RPC 协议）
func Transmission(cfg map[string]string) *Result {
	base := trimBase(cfg["url"])
	if base == "" {
		return errResult("未配置 Transmission 地址")
	}
	endpoint := base + "/transmission/rpc"
	c := newClient(true, 8*time.Second)
	session := ""

	call := func(payload map[string]interface{}) (map[string]interface{}, error) {
		body, _ := json.Marshal(payload)
		for attempt := 0; attempt < 2; attempt++ {
			req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			if session != "" {
				req.Header.Set("X-Transmission-Session-Id", session)
			}
			if u := cfg["username"]; u != "" {
				req.SetBasicAuth(u, cfg["password"])
			}
			resp, err := c.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode == 409 {
				session = resp.Header.Get("X-Transmission-Session-Id")
				resp.Body.Close()
				continue
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			var out map[string]interface{}
			if err := json.Unmarshal(b, &out); err != nil {
				return nil, err
			}
			return out, nil
		}
		return nil, fmt.Errorf("会话协商失败")
	}

	payload := map[string]interface{}{
		"method": "session-stats",
		"arguments": map[string]interface{}{},
	}
	out, err := call(payload)
	if err != nil {
		return errResult("连接失败：%v", err)
	}
	args, _ := out["arguments"].(map[string]interface{})
	if args == nil {
		return errResult("响应格式异常")
	}
	num := func(k string) int64 {
		if v, ok := args[k].(float64); ok {
			return int64(v)
		}
		return 0
	}
	dlSpeed := num("downloadSpeed")
	upSpeed := num("uploadSpeed")
	active := num("activeTorrentCount")
	paused := num("pausedTorrentCount")
	total := num("torrentCount")

	seeding := int64(0)
	if list, ok := args["torrentCount"]; ok {
		_ = list
	}
	// 用 torrent-get 拿种子状态
	tg := map[string]interface{}{
		"method": "torrent-get",
		"arguments": map[string]interface{}{
			"fields": []string{"status", "percentDone"},
		},
	}
	if tOut, err := call(tg); err == nil {
		if tArgs, ok := tOut["arguments"].(map[string]interface{}); ok {
			if arr, ok := tArgs["torrents"].([]interface{}); ok {
				for _, it := range arr {
					m, _ := it.(map[string]interface{})
					if st, ok := m["status"].(float64); ok {
						// 5 做种等待 / 6 做种中
						if int(st) == 5 || int(st) == 6 {
							seeding++
						}
					}
				}
			}
		}
	}

	return &Result{
		OK:   true,
		Text: "↓" + FormatSpeed(uint64(dlSpeed)) + " ↑" + FormatSpeed(uint64(upSpeed)),
		Stats: []Stat{
			{Label: "下载", Value: itoa(int(active)), Tone: tone(active > 0, "warn")},
			{Label: "做种", Value: itoa(int(seeding))},
		},
		Detail: [][]string{
			{"下载速度", FormatSpeed(uint64(dlSpeed))},
			{"上传速度", FormatSpeed(uint64(upSpeed))},
			{"活动任务", itoa(int(active))},
			{"暂停", itoa(int(paused))},
			{"做种中", itoa(int(seeding))},
			{"任务总数", itoa(int(total))},
		},
	}
}

var _ = strings.TrimSpace
