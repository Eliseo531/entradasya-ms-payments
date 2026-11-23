package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// InternalAuth valida estrictamente la presencia y valor de la cabecera X-Internal-Token
func InternalAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Internal-Token")
		if token == "" || token != expectedToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Forbidden",
				"message": "Invalid or missing X-Internal-Token header",
			})
			return
		}
		c.Next()
	}
}
