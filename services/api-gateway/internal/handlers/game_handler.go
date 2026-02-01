package handlers

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	gameServiceURL string
}

func NewGameHandler(host, port string) *GameHandler {
	return &GameHandler{
		gameServiceURL: fmt.Sprintf("http://%s:%s", host, port),
	}
}

func (h *GameHandler) ProxyWebSocket(c *gin.Context) {
	remote, err := url.Parse(h.gameServiceURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse game service URL"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		if userID, exists := c.Get("user_id"); exists {
			req.Header.Set("X-User-ID", userID.(string))
		}

		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = "/ws"
		req.URL.RawQuery = c.Request.URL.RawQuery
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}