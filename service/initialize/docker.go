package initialize

import (
	"context"
	"os"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/lib/dockerApi"
)

// InitDockerClient 初始化 Docker 客户端；socket 不存在时静默跳过
func InitDockerClient() {
	sock := strings.TrimSpace(os.Getenv("DOCKER_SOCK"))
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	if _, err := os.Stat(sock); err != nil {
		if global.Logger != nil {
			global.Logger.Infof("DOCKER MANAGER DISABLED (%s not found)", sock)
		}
		return
	}
	client := dockerApi.NewClient(sock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		if global.Logger != nil {
			global.Logger.Warnf("DOCKER MANAGER PING FAILED: %v", err)
		}
		return
	}
	global.DockerClient = client
	if global.Logger != nil {
		global.Logger.Infof("DOCKER MANAGER CONNECTED via %s", sock)
	}
}
