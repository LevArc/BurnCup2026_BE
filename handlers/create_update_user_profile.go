package handlers

import (
	"net/http"

	"github.com/NotchG/BurnCup/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// CreateUserProfileHandler creates or updates a user profile using JWT claims and request body
func CreateUserProfileHandler(db *sqlx.DB) gin.HandlerFunc {
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
		userEmail, ok := mapClaims["email"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User email not found in token"})
			return
		}

		var req models.User
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Use claims for email and possibly other fields
		req.Email = userEmail

		_, err := db.Exec(
			`INSERT INTO users (email, binusian, full_name, phone_number, nim, major, school)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (email) DO UPDATE SET
				binusian=EXCLUDED.binusian,
				full_name=EXCLUDED.full_name,
				phone_number=EXCLUDED.phone_number,
				nim=EXCLUDED.nim,
				major=EXCLUDED.major,
				school=EXCLUDED.school`,
			req.Email, req.Binusian, req.FullName, req.PhoneNumber, req.NIM, req.Major, req.School,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create or update user"})
			return
		}

		c.JSON(http.StatusCreated, req)
	}
}
