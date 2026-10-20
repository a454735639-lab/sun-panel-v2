package wgt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func embyLike(cfg map[string]string, prefix, label string) *Result {
	base := trimBase(cfg["url"])
	if base == "" {
		return errResult("未配置 %s 地址", label)
	}
	key := strings.TrimSpace(cfg["apikey"])
	if key == "" {
		return errResult("未配置 %s API Key", label)
	}
	c := newClient(true, 8*time.Second)

	get := func(path string) ([]byte, error) {
		req, err := http.NewRequest(http.MethodGet, base+prefix+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Emby-Token", key)
		req.Header.Set("Accept", "application/json")
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

	var counts struct {
		MovieCount   int `json:"MovieCount"`
		SeriesCount  int `json:"SeriesCount"`
		EpisodeCount int `json:"EpisodeCount"`
	}
	if b, err := get("/Items/Counts"); err == nil {
		_ = json.Unmarshal(b, &counts)
	} else {
		return errResult("获取片库失败：%v", err)
	}

	playing := 0
	var nowPlaying []string
	if b, err := get("/Sessions"); err == nil {
		var sessions []struct {
			NowPlayingItem *struct {
				Name string `json:"Name"`
				Type string `json:"Type"`
			} `json:"NowPlayingItem"`
			UserName string `json:"UserName"`
		}
		if json.Unmarshal(b, &sessions) == nil {
			for _, s := range sessions {
				if s.NowPlayingItem != nil && s.NowPlayingItem.Name != "" {
					playing++
					if len(nowPlaying) < 3 {
						nowPlaying = append(nowPlaying, s.NowPlayingItem.Name)
					}
				}
			}
		}
	}

	text := "空闲"
	if playing > 0 {
		text = fmt.Sprintf("%d 个会话播放中", playing)
		if len(nowPlaying) > 0 {
			text = "正在播放：" + nowPlaying[0]
		}
	}

	res := &Result{
		OK:   true,
		Text: text,
		Stats: []Stat{
			{Label: "播放中", Value: itoa(playing), Tone: tone(playing > 0, "good")},
			{Label: "电影", Value: itoa(counts.MovieCount)},
			{Label: "剧集", Value: itoa(counts.SeriesCount)},
		},
		Detail: [][]string{
			{"播放中会话", itoa(playing)},
			{"电影", itoa(counts.MovieCount)},
			{"剧集", itoa(counts.SeriesCount)},
			{"单集", itoa(counts.EpisodeCount)},
		},
	}
	for i, n := range nowPlaying {
		res.Detail = append(res.Detail, []string{fmt.Sprintf("播放 %d", i+1), n})
	}
	return res
}

// Emby Emby 小组件
func Emby(cfg map[string]string) *Result { return embyLike(cfg, "/emby", "Emby") }

// Jellyfin Jellyfin 小组件
func Jellyfin(cfg map[string]string) *Result { return embyLike(cfg, "", "Jellyfin") }
