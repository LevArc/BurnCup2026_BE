package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// GetQRLinkHandler returns a link to the QR code generator handler with value 'qr-teamCode-currentUnixTime'
// and updates the order_id in registered_competitions for the given teamCode to this value
func GetQRLinkHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamCode := c.Param("teamCode")
		if teamCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing teamCode parameter"})
			return
		}

		// Check if there's an existing valid QR
		var existingQR struct {
			QrURL     *string    `db:"qr_url"`
			ValidTime *time.Time `db:"valid_time"`
		}

		err := db.Get(&existingQR, `
            SELECT qr_url, valid_time 
            FROM registered_competitions 
            WHERE team_code = $1
        `, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Print debug info: local time and valid time
		fmt.Printf("DEBUG: Local time: %s\n", time.Now().Format(time.RFC3339))
		if existingQR.ValidTime != nil {
			fmt.Printf("DEBUG: Valid time: %s\n", existingQR.ValidTime.Add(-7*time.Hour).Format(time.RFC3339))
		} else {
			fmt.Println("DEBUG: Valid time is nil")
		}

		// If QR exists and is still valid, return it
		if existingQR.QrURL != nil && existingQR.ValidTime != nil {
			expiryUTC := existingQR.ValidTime.Add(-7 * time.Hour)
			if time.Now().Before(expiryUTC) {
				c.JSON(http.StatusOK, gin.H{
					"qrLink":     *existingQR.QrURL,
					"expiryTime": expiryUTC.Format(time.RFC3339),
				})
				return
			}
		}

		// QR doesn't exist or is expired, create a new one
		now := strconv.FormatInt(time.Now().Unix(), 10)
		qrValue := fmt.Sprintf("qr-%s-%s", teamCode, now)

		// Get registration_fee from the competition
		var registrationFee int
		err = db.Get(&registrationFee, `
            SELECT c.registration_fee 
            FROM competitions c
            JOIN registered_competitions rc ON c.id = rc.competition_id
            WHERE rc.team_code = $1
        `, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team or competition not found"})
			return
		}

		// Calculate total amount with 0.7% fee
		totalAmount := float64(registrationFee) * 1.007

		// Create Midtrans QR and get response with URL and expiry time
		qrResponse, err := CreateMidtransQR(db, qrValue, totalAmount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Midtrans QR"})
			return
		}

		// Update order_id, qr_url, and valid_time in registered_competitions
		_, err = db.Exec(`
            UPDATE registered_competitions 
            SET order_id = $1, qr_url = $2, valid_time = $3, updated_at = NOW()
            WHERE team_code = $4`,
			qrValue, qrResponse.URL, qrResponse.ExpiryTime, teamCode,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update registration data"})
			return
		}

		// Subtract 7 hours from expiry time to convert from +7 timezone to UTC
		expiryUTC := qrResponse.ExpiryTime.Add(-7 * time.Hour)
		c.JSON(http.StatusOK, gin.H{
			"qrLink":     qrResponse.URL,
			"expiryTime": expiryUTC.Format(time.RFC3339),
		})
	}
}
