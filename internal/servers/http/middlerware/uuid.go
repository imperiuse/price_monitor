package middlerware

import (
	"github.com/gin-gonic/gin"

	"github.com/imperiuse/price_monitor/internal/uuid"
)

const XRequestID = "X-Request-Id"

func UUIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := uuid.UUID4()
		c.Set(XRequestID, u)
		c.Writer.Header().Set(XRequestID, u)

		c.Next()
	}
}
