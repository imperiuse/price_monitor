package middlerware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware - cors middleware.
func CORSMiddleware(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type,"+
			" Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding,"+
			" Accept-Language, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)

			return
		}

		c.Request.Header.Del("Origin")

		c.Next()
	}
}
