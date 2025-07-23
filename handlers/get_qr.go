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

		// Check team exists and get competition info
		var teamInfo struct {
			TeamID        string     `db:"team_id"`
			CompetitionID string     `db:"competition_id"`
			MinMembers    *int       `db:"min_members"`
			TeamSlot      int        `db:"team_slot"`
			QrURL         *string    `db:"qr_url"`
			ValidTime     *time.Time `db:"valid_time"`
		}

		err := db.Get(&teamInfo, `
            SELECT rc.id as team_id, rc.competition_id, c.min_members, c.team_slot, rc.qr_url, rc.valid_time
            FROM registered_competitions rc
            JOIN competitions c ON c.id = rc.competition_id
            WHERE rc.team_code = $1
        `, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Check if team meets minimum member requirements
		var currentMembers int
		err = db.Get(&currentMembers, `
            SELECT COUNT(*) FROM registered_competition_members
            WHERE registered_competition_id = $1
        `, teamInfo.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count team members"})
			return
		}

		if teamInfo.MinMembers != nil && currentMembers + 1 < *teamInfo.MinMembers {
			c.JSON(http.StatusForbidden, gin.H{
				"error":           fmt.Sprintf("Team must have at least %d members to proceed with payment", *teamInfo.MinMembers),
				"currentMembers":  currentMembers,
				"requiredMembers": *teamInfo.MinMembers,
			})
			return
		}

		// Check if team slots are full (only count paid teams)
		var paidTeams int
		err = db.Get(&paidTeams, `
            SELECT COUNT(*) FROM registered_competitions 
            WHERE competition_id = $1 AND is_paid = true
        `, teamInfo.CompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team slots"})
			return
		}

		if paidTeams >= teamInfo.TeamSlot {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Competition team slots are full",
				"availableSlots": 0,
				"totalSlots":     teamInfo.TeamSlot,
			})
			return
		}

		// Print debug info: local time and valid time
		fmt.Printf("DEBUG: Local time: %s\n", time.Now().Format(time.RFC3339))
		if teamInfo.ValidTime != nil {
			fmt.Printf("DEBUG: Valid time: %s\n", teamInfo.ValidTime.Add(-7*time.Hour).Format(time.RFC3339))
		} else {
			fmt.Println("DEBUG: Valid time is nil")
		}

		// If QR exists and is still valid, return it
		if teamInfo.QrURL != nil && teamInfo.ValidTime != nil {
			expiryUTC := teamInfo.ValidTime.Add(-7 * time.Hour)
			if time.Now().Before(expiryUTC) {
				c.JSON(http.StatusOK, gin.H{
					"qrLink":     *teamInfo.QrURL,
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
