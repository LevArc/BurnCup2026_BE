package handlers

import (
	"net/http"

	"github.com/NotchG/BurnCup/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// GetCurrentUserHandler returns the current user based on JWT claims
func GetCurrentUserHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get("user")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No user claims found"})
			return
		}

		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user claims"})
			return
		}
		userID, ok := mapClaims["sub"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		var user models.User
		if err := db.Get(&user, `SELECT id, binusian, full_name, phone_number, email, nim, major, school FROM users WHERE id=$1`, userID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}
