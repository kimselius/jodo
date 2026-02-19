package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// jodoProxy returns a handler that reverse-proxies requests to Jodo's app.
// Requests to /jodo/* are forwarded to http://<jodo-host>:<app-port>/ with
// the /jodo prefix stripped.
func (s *Server) jodoProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.Config == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "not configured"})
			return
		}

		host := s.Config.Jodo.Host
		port := s.Config.Jodo.AppPort
		target, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "bad proxy target"})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Strip /jodo prefix before forwarding
		originalPath := c.Request.URL.Path
		c.Request.URL.Path = strings.TrimPrefix(originalPath, "/jodo")
		if c.Request.URL.Path == "" {
			c.Request.URL.Path = "/"
		}

		// Don't let proxy errors crash the kernel
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "Jodo's app is not responding",
			})
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
