package pages

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"sun-panel/global"
	"sun-panel/lib/wgt"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
)

type widgetCacheEntry struct {
	res *wgt.Result
	at  time.Time
}

var (
	widgetMu    sync.RWMutex
	widgetCache = map[uint]widgetCacheEntry{}
)

const (
	widgetOKTTL = 20 * time.Second
	widgetErrTTL = 5 * time.Second
)

func invalidateWidgetCache(cardID uint) {
	widgetMu.Lock()
	delete(widgetCache, cardID)
	widgetMu.Unlock()
}

// WidgetBatch 批量获取小组件数据
func WidgetBatch(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	var req struct {
		CardIds []uint `json:"cardIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	if len(req.CardIds) > 40 {
		req.CardIds = req.CardIds[:40]
	}

	cards := []models.ItemIcon{}
	if len(req.CardIds) > 0 && global.Db != nil {
		global.Db.Where("user_id = ? AND id IN ?", user.ID, req.CardIds).Find(&cards)
	}

	type item struct {
		CardID uint        `json:"cardId"`
		OK     bool        `json:"ok"`
		Error  string      `json:"error,omitempty"`
		Text   string      `json:"text,omitempty"`
		Stats  []wgt.Stat  `json:"stats,omitempty"`
		Detail [][]string  `json:"detail,omitempty"`
	}

	out := make([]item, 0, len(cards))
	for i := range cards {
		card := cards[i]
		if card.WidgetType == "" {
			continue
		}
		// 缓存
		widgetMu.RLock()
		e, hit := widgetCache[card.ID]
		widgetMu.RUnlock()
		var res *wgt.Result
		if hit {
			ttl := widgetOKTTL
			if !e.res.OK {
				ttl = widgetErrTTL
			}
			if time.Since(e.at) < ttl {
				res = e.res
			}
		}
		if res == nil {
			cfg := map[string]string{}
			if card.WidgetConfig != "" {
				_ = json.Unmarshal([]byte(card.WidgetConfig), &cfg)
			}
			res = wgt.Run(card.WidgetType, cfg)
			widgetMu.Lock()
			widgetCache[card.ID] = widgetCacheEntry{res: res, at: time.Now()}
			widgetMu.Unlock()
		}

		out = append(out, item{
			CardID: card.ID,
			OK:     res.OK,
			Error:  res.Error,
			Text:   res.Text,
			Stats:  res.Stats,
			Detail: res.Detail,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"list": out}})
}

// WidgetTypes 可用小组件类型
func WidgetTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wgt.Types()})
}
