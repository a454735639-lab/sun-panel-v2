package initialize

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sun-panel/global"
	"sun-panel/lib/siteFavicon"
	"sun-panel/models"
	"sun-panel/models/datatype"
	"time"
)

// UpdateIconBase64 更新item_icon表中的非base64格式图标
func UpdateIconBase64() {
	global.Logger.Info("Start updating non-base64 icons...")

	// 获取所有item_icon记录
	var itemIcons []models.ItemIcon
	if err := global.Db.Find(&itemIcons).Error; err != nil {
		global.Logger.Error("Failed to get item icons:", err)
		return
	}

	updatedCount := 0
	skippedCount := 0
	errorCount := 0

	// 遍历所有记录
	for _, itemIcon := range itemIcons {
		// 解析IconJson为ItemIconIconInfo
		var iconInfo datatype.ItemIconIconInfo
		if err := json.Unmarshal([]byte(itemIcon.IconJson), &iconInfo); err != nil {
			global.Logger.Error("Failed to parse IconJson:", err)
			errorCount++
			continue
		}

		// 仅在图标为空时才自动抓取。
		// 绝不覆盖用户自定义的图标（外部 URL 或本地 /uploads 路径），
		// 否则用户精心挑选的图标会被自动抓取的通用 favicon 顶掉。
		if iconInfo.Src != "" {
			skippedCount++
			continue
		}

		// 获取base64格式的图标
		base64Icon, err := getIconBase64(itemIcon.Url)
		if err != nil {
			global.Logger.Error("Failed to get base64 icon for URL", itemIcon.Url, ":", err)
			errorCount++
			continue
		}

		// 更新图标信息
		iconInfo.Src = base64Icon
		// 必须同时把 ItemType 标成「图片」。
		//
		// 前端按 itemType 分支渲染：1=文字 / 2=图片 / 3=在线图标 / 其他=兜底首字。
		// 只填 Src 而不改 ItemType，会出现"库里明明有图标数据、
		// 页面却显示首字占位"的怪象 —— 数据看着是好的，肉眼排查极难。
		if iconInfo.ItemType != 1 && iconInfo.ItemType != 2 && iconInfo.ItemType != 3 {
			iconInfo.ItemType = 2
		}
		updatedIconJson, err := json.Marshal(iconInfo)
		if err != nil {
			global.Logger.Error("Failed to marshal updated icon info:", err)
			errorCount++
			continue
		}

		// 保存到数据库
		if err := global.Db.Model(&itemIcon).Update("icon_json", string(updatedIconJson)).Error; err != nil {
			global.Logger.Error("Failed to update icon:", err)
			errorCount++
			continue
		}

		updatedCount++
		global.Logger.Info("Updated icon for URL:", itemIcon.Url)
	}

	global.Logger.Info("Icon update completed. Updated:", updatedCount, "Skipped:", skippedCount, "Errors:", errorCount)
}

// getIconBase64 获取URL的base64格式图标
func getIconBase64(urlStr string) (string, error) {
	// 首先解析URL，获取域名和协议
	parsedURL, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return "", fmt.Errorf("failed to parse URL: %w", parseErr)
	}
	domain := parsedURL.Host

	// 1. 首先尝试从网站本身获取favicon
	iconUrl, err := siteFavicon.GetOneFaviconURL(urlStr)
	if err != nil {
		// 2. 网站本身取不到，改用第三方服务。
		//    与 GetSiteFavicon 保持同一套回退顺序：大陆可达优先、Google 兜底。
		global.Logger.Debug("Failed to get favicon from site, trying fallback services:", err)

		for _, fb := range siteFavicon.FallbackFaviconURLs(domain) {
			b64, e := getImageBase64(fb)
			if e == nil && b64 != "" {
				return b64, nil
			}
			global.Logger.Debug("Fallback favicon service failed:", fb, e)
		}
		return "", fmt.Errorf("no favicon from site or any fallback service")
	}

	// 确保URL格式正确
	if !strings.HasPrefix(iconUrl, "http://") && !strings.HasPrefix(iconUrl, "https://") {
		protocol := parsedURL.Scheme

		if strings.HasPrefix(iconUrl, "/") {
			// 绝对路径，添加协议和域名
			iconUrl = protocol + "://" + domain + iconUrl
		} else if strings.HasPrefix(iconUrl, "//") {
			// 双斜杠开头，添加协议
			iconUrl = protocol + ":" + iconUrl
		} else {
			// 相对路径，添加协议、域名和斜杠
			iconUrl = protocol + "://" + domain + "/" + iconUrl
		}
	}

	// 去掉无意义的查询参数，但保留第三方 favicon 服务的 domain 参数 ——
	// 一律剥掉会把回退 URL 变成"服务首页"，拿不到目标网站的图标。
	iconUrl = siteFavicon.StripQuery(iconUrl)

	// 将图片转换为base64
	base64Icon, err := getImageBase64(iconUrl)
	if err != nil {
		// 从网站下载失败，逐个尝试第三方服务。
		// 不能只押注 Google —— google.com 在中国大陆不可达，
		// 那样等于"回退也必然失败"，且启动日志里看不出是这一步挂了。
		global.Logger.Debug("Failed to download or convert favicon, trying fallback services:", err)

		var lastErr error = err
		for _, fb := range siteFavicon.FallbackFaviconURLs(domain) {
			b64, e := getImageBase64(fb)
			if e == nil && b64 != "" {
				base64Icon = b64
				err = nil
				break
			}
			lastErr = e
			global.Logger.Debug("Fallback favicon service failed:", fb, e)
		}
		if err != nil {
			return "", fmt.Errorf("failed to get favicon from all sources: %w", lastErr)
		}
	}

	return base64Icon, nil
}

// getImageBase64 将远程图片转换为base64
func getImageBase64(imageUrl string) (string, error) {
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second, // 10秒超时
	}

	// 发送HTTP GET请求
	resp, err := client.Get(imageUrl)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get image, status code: %d", resp.StatusCode)
	}

	// 读取响应体
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, resp.Body); err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// 获取Content-Type
	contentType := resp.Header.Get("Content-Type")

	// 必须确认拿到的是图片。
	//
	// 原因：很多站点（SPA 兜底路由、CDN）对任意路径都返回 200 + HTML，
	// 于是 `GET /favicon.ico` 会"成功"返回一个 HTML 页面。
	// 原实现不校验类型，就把这段 HTML 原样 base64 编码存进 icon_json ——
	// 前端 `data:text/html;base64,...` 塞进 <img> 必然破图，
	// 表现为"图标获取不到"，而后端日志显示一切正常。
	if !strings.HasPrefix(contentType, "image/") {
		head := buffer.Bytes()
		if len(head) > 256 {
			head = head[:256]
		}
		return "", fmt.Errorf("not an image: content-type=%q body-preview=%q",
			contentType, string(head))
	}

	// 转换为base64
	base64Str := base64.StdEncoding.EncodeToString(buffer.Bytes())

	// 返回data URL格式
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Str), nil
}

// UpdateBookmarkParentId 更新bookmark表中的parent_id字段，将老数据的parent_url转换为parent_id
func UpdateBookmarkParentId() {
	global.Logger.Info("Start updating bookmark parent_id...")

	// 1. 获取所有bookmark记录
	var allBookmarks []models.Bookmark
	if err := global.Db.Find(&allBookmarks).Error; err != nil {
		global.Logger.Error("Failed to get all bookmarks:", err)
		return
	}

	// 2. 按用户分组处理
	userBookmarks := make(map[uint][]models.Bookmark)
	for _, bookmark := range allBookmarks {
		userBookmarks[bookmark.UserId] = append(userBookmarks[bookmark.UserId], bookmark)
	}

	for userId, bookmarks := range userBookmarks {
		global.Logger.Info("Processing bookmarks for user:", userId)

		// 3. 构建文件夹标题到ID的映射，处理同名文件夹
		titleToIdMap := make(map[string]uint)
		folderTitleCount := make(map[string]int)
		folders := make([]models.Bookmark, 0)

		// 统计每个文件夹标题的出现次数
		for _, bookmark := range bookmarks {
			if bookmark.IsFolder == 1 {
				folderTitleCount[bookmark.Title]++
				folders = append(folders, bookmark)
			}
		}

		// 处理同名文件夹，保留一个，删除其他的
		for _, bookmark := range folders {
			if bookmark.IsFolder == 1 {
				if _, exists := titleToIdMap[bookmark.Title]; !exists {
					// 保留第一个出现的文件夹
					titleToIdMap[bookmark.Title] = bookmark.ID
					global.Logger.Info("Keeping folder:", bookmark.Title, "ID:", bookmark.ID)

					// 如果有多个同名文件夹，删除其他的
					if folderTitleCount[bookmark.Title] > 1 {
						// 查找并删除其他同名文件夹
						var duplicateFolders []models.Bookmark
						if err := global.Db.Where("user_id = ? AND is_folder = ? AND title = ? AND id != ?",
							userId, 1, bookmark.Title, bookmark.ID).Find(&duplicateFolders).Error; err != nil {
							global.Logger.Error("Failed to find duplicate folders:", err)
							continue
						}

						for _, dupFolder := range duplicateFolders {
							// 1. 更新该文件夹下所有子项的parent_url到保留的文件夹
							global.Db.Model(&models.Bookmark{}).Where("user_id = ? AND parent_url = ?",
								userId, dupFolder.Title).Update("parent_url", bookmark.Title)

							// 2. 删除重复文件夹
							if err := global.Db.Delete(&dupFolder).Error; err != nil {
								global.Logger.Error("Failed to delete duplicate folder:", dupFolder.ID, err)
							} else {
								global.Logger.Info("Deleted duplicate folder:", dupFolder.Title, "ID:", dupFolder.ID)
							}
						}
					}
				}
			}
		}

		// 4. 更新所有书签的parent_id
		updatedCount := 0
		for _, bookmark := range bookmarks {
			if bookmark.ParentUrl != "0" && bookmark.ParentUrl != "" && bookmark.ParentUrl != "null" {
				if parentId, exists := titleToIdMap[bookmark.ParentUrl]; exists {
					// 更新parent_id
					if err := global.Db.Model(&models.Bookmark{}).Where("id = ? AND user_id = ?",
						bookmark.ID, userId).Update("parent_id", parentId).Error; err != nil {
						global.Logger.Error("Failed to update parent_id for bookmark:", bookmark.ID, err)
					} else {
						updatedCount++
						global.Logger.Info("Updated bookmark:", bookmark.ID, "parent_id:", parentId)
					}
				}
			}
		}

		global.Logger.Info("Updated parent_id for", updatedCount, "bookmarks for user:", userId)
	}

	global.Logger.Info("Bookmark parent_id update completed.")
}

// UpdateBookmarkIconJson 更新bookmark表中icon_json为空的数据，使用谷歌API获取图标
func UpdateBookmarkIconJson() {
	global.Logger.Info("Start updating bookmark icon_json for empty values...")

	// 获取所有bookmark记录
	var bookmarks []models.Bookmark
	if err := global.Db.Find(&bookmarks).Error; err != nil {
		global.Logger.Error("Failed to get all bookmarks:", err)
		return
	}

	updatedCount := 0
	skippedCount := 0
	errorCount := 0

	// 遍历所有记录
	for _, bookmark := range bookmarks {
		// 只处理非文件夹且icon_json为空的书签
		if bookmark.IsFolder == 1 {
			skippedCount++
			continue
		}

		// 检查icon_json是否为空
		if bookmark.IconJson != "" && bookmark.IconJson != "null" && bookmark.IconJson != "{}" {
			skippedCount++
			continue
		}

		// 检查URL是否为空
		if bookmark.Url == "" {
			skippedCount++
			continue
		}

		// 获取base64格式的图标
		base64Icon, err := getIconBase64(bookmark.Url)
		if err != nil {
			global.Logger.Error("Failed to get icon for URL", bookmark.Url, "ID", bookmark.ID, ":", err)
			errorCount++
			continue
		}

		// 构建icon_json
		iconInfo := datatype.ItemIconIconInfo{
			ItemType:        1, // 1表示书签图标
			Src:             base64Icon,
			Text:            "",
			BackgroundColor: "",
		}

		iconJson, err := json.Marshal(iconInfo)
		if err != nil {
			global.Logger.Error("Failed to marshal icon info for bookmark", bookmark.ID, ":", err)
			errorCount++
			continue
		}

		// 更新数据库
		if err := global.Db.Model(&bookmark).Update("icon_json", string(iconJson)).Error; err != nil {
			global.Logger.Error("Failed to update icon_json for bookmark", bookmark.ID, ":", err)
			errorCount++
			continue
		}

		updatedCount++
		global.Logger.Info("Updated icon_json for bookmark", bookmark.ID, "URL:", bookmark.Url)
	}

	global.Logger.Info("Bookmark icon_json update completed. Updated:", updatedCount, "Skipped:", skippedCount, "Errors:", errorCount)
}
