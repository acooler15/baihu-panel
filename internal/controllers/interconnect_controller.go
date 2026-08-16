package controllers

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/tunnel"
	"github.com/engigu/baihu-panel/internal/utils"

	"github.com/gin-gonic/gin"
)

type InterconnectController struct {
	interconnectService *services.InterconnectService
	httpClient          *http.Client
}

func NewInterconnectController(interconnectService *services.InterconnectService) *InterconnectController {
	return &InterconnectController{
		interconnectService: interconnectService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HandleTunnel 接受子节点 WebSocket 连接请求
func (ic *InterconnectController) HandleTunnel(c *gin.Context) {
	tunnel.HandleTunnel(c)
}

// ProxyRequest 代理转发请求至目标节点
func (ic *InterconnectController) ProxyRequest(c *gin.Context) {
	nodeID := c.Param("node_id")
	path := c.Param("path")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: "Node ID required"})
		return
	}

	node, err := ic.interconnectService.GetNodeByID(nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.Response{Code: 404, Msg: "Node not found"})
		return
	}

	if strings.HasPrefix(node.URL, "tunnel://") {
		// 走 WebSocket 逆向隧道 (基于 Yamux 流式多路复用)
		err := tunnel.ProxyHTTP(nodeID, c, path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Tunnel request failed: " + err.Error()})
		}
		return
	}

	// 走普通 HTTP 直连
	// Construct the target URL
	targetURL := strings.TrimRight(node.URL, "/") + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Failed to create proxy request"})
		return
	}

	// Copy headers
	req.Header = c.Request.Header.Clone()

	// If the node token exists, append it as Bearer Auth
	if node.Token != "" {
		req.Header.Set("Authorization", "Bearer "+node.Token)
	}

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "Failed to connect to target node: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// ReportMonitorData 接收子节点上报的监控数据
func (ic *InterconnectController) ReportMonitorData(c *gin.Context) {
	var req models.NodeMetrics

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.Response{Code: 400, Msg: err.Error()})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
		return
	}
	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	node, err := ic.interconnectService.GetNodeByToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	err = ic.interconnectService.UpdateNodeMonitorData(node.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.Response{Code: 500, Msg: "更新节点数据失败"})
		return
	}
	c.JSON(http.StatusOK, utils.Response{
		Code: 200,
		Msg:  "success",
		Data: gin.H{
			"tunnel_url": node.URL,
		},
	})
}

// getClientAndURL 辅助方法：根据节点类型决定走直连还是隧道，并返回对应的 Client 和完整 URL
func (ic *InterconnectController) getClientAndURL(node *models.InterconnectNode, path string) (*http.Client, string, error) {
	if strings.HasPrefix(node.URL, "tunnel://") {
		sess := tunnel.GetSession(node.ID)
		if sess == nil {
			return nil, "", net.ErrClosed
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sess.Session.Open()
			},
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}
		return client, "http://tunnel.local" + path, nil
	}

	targetURL := strings.TrimRight(node.URL, "/") + path
	return ic.httpClient, targetURL, nil
}
