package models

import "encoding/json"

// DockerExpandParam Docker 卡片的扩展参数
type DockerExpandParam struct {
	ContainerId   string `json:"containerId"`
	ContainerName string `json:"containerName"`
}

// GetDockerExpandParam 解析 Docker 扩展参数，非 Docker 卡片返回 nil
func (m *ItemIcon) GetDockerExpandParam() *DockerExpandParam {
	if m.ExpandParam == "" {
		return nil
	}
	var p DockerExpandParam
	if err := json.Unmarshal([]byte(m.ExpandParam), &p); err != nil {
		return nil
	}
	if p.ContainerId == "" {
		return nil
	}
	return &p
}
