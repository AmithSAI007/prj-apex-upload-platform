package middleware

import "github.com/gin-gonic/gin"

const (
	// CORSHeaders defines the HTTP headers used for CORS responses.
	CORSHeaders = "Content-Type, Authorization"
	// CORSMethods defines the HTTP methods allowed for CORS requests.
	CORSMethods = "POST, OPTIONS, GET, PUT"
)

// CORSMiddleware returns a Gin middleware handler that sets CORS headers on
// responses and handles preflight OPTIONS requests. It allows all origins and
// supports credentials, specific headers, and methods.
func CORSMiddleware(origins []string, allowCredentials bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		originHeader := ""
		for i, origin := range origins {
			if i > 0 {
				originHeader += ", "
			}
			originHeader += origin
		}

		allowCredentialsValue := "false"
		if allowCredentials {
			allowCredentialsValue = "true"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", originHeader)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", allowCredentialsValue)
		c.Writer.Header().Set("Access-Control-Allow-Headers", CORSHeaders)
		c.Writer.Header().Set("Access-Control-Allow-Methods", CORSMethods)

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
