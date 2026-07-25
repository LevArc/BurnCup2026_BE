package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5" // Ensure you have this package or your equivalent JWT package
)

// Define your list of authorized admin emails here
var adminEmails = map[string]bool{
	"edward.matthew.tenggono@gmail.com": true,
	"ghanifabihaziq@gmail.com": true,
	"burncupbinusbekasi@gmail.com":true,

}

// AdminAuthMiddleware validates the JWT and checks if the user's email is in the admin list
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Use the same JWT secret used during your login/registration
			return []byte(os.Getenv("JWT_SECRET_KEY")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Extract email (ensure "email" matches the key used when generating the token)
		emailClaim, ok := claims["email"]
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Email not found in token"})
			c.Abort()
			return
		}

		email := emailClaim.(string)

		// Check if the email exists in our admin list
		if !adminEmails[email] {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: Admin privileges required"})
			c.Abort()
			return
		}

		// Optional: Store the admin email in the context for handlers to use
		c.Set("admin_email", email)

		c.Next()
	}
}
