package dockerApi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client 通过 unix socket 访问 Docker Engine API
type Client struct {
	httpc *http.Client
}

// NewClient 创建客户端；sock 为 unix socket 路径
func NewClient(sock string) *Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
		MaxIdleConns:    4,
		IdleConnTimeout: 30 * time.Second,
	}
	return &Client{httpc: &http.Client{Transport: tr, Timeout: 20 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	u := "http://docker" + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return b, resp.StatusCode, fmt.Errorf("docker api %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, resp.StatusCode, nil
}

// Ping 检查连通性
func (c *Client) Ping(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodGet, "/_ping", nil)
	return err
}

// ContainerInfo 容器概要
type ContainerInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Created     int64             `json:"created"`
	Ports       []PortMapping     `json:"ports"`
	PrimaryPort int               `json:"primaryPort"`
	Labels      map[string]string `json:"labels"`
	NavName     string            `json:"navName"`
	Running     bool              `json:"running"`
	// 以下字段由 GetContainerStates 回填，便于模板直接取用
	CPUPercent float64 `json:"cpuPercent"`
	MemUsage   uint64  `json:"memUsage"`
}

// PortMapping 端口映射
type PortMapping struct {
	Private int    `json:"private"`
	Public  int    `json:"public"`
	Type    string `json:"type"`
}

// ContainerState 实时状态与资源占用
type ContainerState struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	State      string  `json:"state"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpuPercent"`
	MemUsage   uint64  `json:"memUsage"`
	MemLimit   uint64  `json:"memLimit"`
	NetRx      uint64  `json:"netRx"`
	NetTx      uint64  `json:"netTx"`
	Running    bool    `json:"running"`
}

type apiContainerListItem struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created int64             `json:"Created"`
	Ports   []apiPort         `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
}

type apiPort struct {
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

// webPorts 常见 Web 端口，优先作为主端口
var webPorts = map[int]bool{
	80: true, 443: true, 3000: true, 3001: true, 5000: true, 5001: true,
	8000: true, 8001: true, 8080: true, 8081: true, 8088: true, 8090: true,
	8096: true, 8443: true, 8888: true, 9000: true, 9090: true, 32400: true,
}

func pickPrimaryPort(ports []PortMapping) int {
	best := 0
	for _, p := range ports {
		if p.Public == 0 {
			continue
		}
		if webPorts[p.Public] || webPorts[p.Private] {
			return p.Public
		}
		if best == 0 {
			best = p.Public
		}
	}
	return best
}

// ListContainers 列出全部容器
func (c *Client) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	b, _, err := c.do(ctx, http.MethodGet, "/containers/json?all=true", nil)
	if err != nil {
		return nil, err
	}
	var raw []apiContainerListItem
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := make([]ContainerInfo, 0, len(raw))
	for _, it := range raw {
		ports := make([]PortMapping, 0, len(it.Ports))
		for _, p := range it.Ports {
			ports = append(ports, PortMapping{Private: p.PrivatePort, Public: p.PublicPort, Type: p.Type})
		}
		name := ""
		if len(it.Names) > 0 {
			name = strings.TrimPrefix(it.Names[0], "/")
		}
		navName := ""
		if it.Labels != nil {
			navName = it.Labels["nav.name"]
		}
		out = append(out, ContainerInfo{
			ID:          it.ID,
			Name:        name,
			Image:       it.Image,
			State:       it.State,
			Status:      it.Status,
			Created:     it.Created,
			Ports:       ports,
			PrimaryPort: pickPrimaryPort(ports),
			Labels:      it.Labels,
			NavName:     navName,
			Running:     it.State == "running",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type apiStats struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func calcCPU(s *apiStats) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	if sysDelta <= 0 || cpuDelta < 0 {
		return 0
	}
	online := float64(s.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if online == 0 {
		online = 1
	}
	return (cpuDelta / sysDelta) * online * 100
}

// GetContainerStates 并发查询容器实时状态
func (c *Client) GetContainerStates(ctx context.Context, ids []string, withStats bool) []ContainerState {
	if len(ids) == 0 {
		return nil
	}
	type res struct {
		idx int
		st  ContainerState
	}
	ch := make(chan res, len(ids))
	sem := make(chan struct{}, 8)
	for i, id := range ids {
		go func(idx int, cid string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			st := ContainerState{ID: cid}
			if withStats {
				p := "/containers/" + url.PathEscape(cid) + "/stats?stream=false&one-shot=true"
				if b, _, err := c.do(ctx, http.MethodGet, p, nil); err == nil {
					var s apiStats
					if json.Unmarshal(b, &s) == nil {
						st.Name = strings.TrimPrefix(s.Name, "/")
						st.CPUPercent = calcCPU(&s)
						st.MemUsage = s.MemoryStats.Usage
						st.MemLimit = s.MemoryStats.Limit
						for _, n := range s.Networks {
							st.NetRx += n.RxBytes
							st.NetTx += n.TxBytes
						}
					}
				}
			}
			// 补充 state/status
			p := "/containers/" + url.PathEscape(cid) + "/json"
			if b, _, err := c.do(ctx, http.MethodGet, p, nil); err == nil {
				var info struct {
					Name  string `json:"Name"`
					State struct {
						Status string `json:"Status"`
					} `json:"State"`
				}
				if json.Unmarshal(b, &info) == nil {
					if st.Name == "" {
						st.Name = strings.TrimPrefix(info.Name, "/")
					}
					st.State = info.State.Status
					st.Status = info.State.Status
					st.Running = info.State.Status == "running"
				}
			}
			ch <- res{idx: idx, st: st}
		}(i, id)
	}

	out := make([]ContainerState, len(ids))
	for range ids {
		r := <-ch
		out[r.idx] = r.st
	}
	return out
}

func (c *Client) action(ctx context.Context, id, verb string) error {
	_, _, err := c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/"+verb, nil)
	return err
}

// StartContainer 启动
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "start")
}

// StopContainer 停止
func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "stop?t=10")
}

// RestartContainer 重启
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "restart?t=10")
}
