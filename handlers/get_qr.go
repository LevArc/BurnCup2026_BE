package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// GetQRLinkHandler returns a QR link for payment, checking slots based on team leader user type
func GetQRLinkHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamCode := c.Param("teamCode")
		if teamCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing teamCode parameter"})
			return
		}

		// Get team info
		var teamInfo struct {
			TeamID        string     `db:"team_id"`
			CompetitionID string     `db:"competition_id"`
			MinMembers    *int       `db:"min_members"`
			QrURL         *string    `db:"qr_url"`
			ValidTime     *time.Time `db:"valid_time"`
		}
		err := db.Get(&teamInfo, `
			SELECT rc.id as team_id, rc.competition_id, c.min_members, rc.qr_url, rc.valid_time
			FROM registered_competitions rc
			JOIN competitions c ON c.id = rc.competition_id
			WHERE rc.team_code = $1
		`, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Check minimum member requirements
		var currentMembers int
		err = db.Get(&currentMembers, `
			SELECT COUNT(*) FROM registered_competition_members
			WHERE registered_competition_id = $1
		`, teamInfo.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count team members"})
			return
		}
		if teamInfo.MinMembers != nil && currentMembers < *teamInfo.MinMembers {
			c.JSON(http.StatusForbidden, gin.H{
				"error":           fmt.Sprintf("Team must have at least %d members to proceed with payment", *teamInfo.MinMembers),
				"currentMembers":  currentMembers,
				"requiredMembers": *teamInfo.MinMembers,
			})
			return
		}

		// Get team leader user type
		var leaderUserType string
		err = db.QueryRowx(`
			SELECT COALESCE(u.user_type, '')
			FROM users u
			JOIN registered_competition_members rcm ON u.id = rcm.user_id
			WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = true
			LIMIT 1
		`, teamInfo.TeamID).Scan(&leaderUserType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team leader info"})
			return
		}

		// Determine slot column based on leader user type
		slotColumn := "non_binusian_team_slot"
		if strings.EqualFold(leaderUserType, "Binusian") {
			slotColumn = "binusian_team_slot"
		}

		// Get total slot for this category
		var totalSlot int
		err = db.QueryRowx(
			fmt.Sprintf(`SELECT %s FROM competition_slots WHERE competition_id = $1`, slotColumn),
			teamInfo.CompetitionID,
		).Scan(&totalSlot)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Slot info not found for this competition"})
			return
		}

		// Count paid teams in same category
		categoryFilter := "LOWER(u.user_type) != 'binusian'"
		if strings.EqualFold(leaderUserType, "Binusian") {
			categoryFilter = "LOWER(u.user_type) = 'binusian'"
		}
		var paidTeams int
		err = db.Get(&paidTeams, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM registered_competitions rc
			JOIN registered_competition_members rcm ON rc.id = rcm.registered_competition_id
			JOIN users u ON u.id = rcm.user_id
			WHERE rc.competition_id = $1
			  AND rc.is_paid = true
			  AND rcm.is_team_leader = true
			  AND %s
		`, categoryFilter), teamInfo.CompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team slots"})
			return
		}
		if paidTeams >= totalSlot {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Competition team slots are full for your category",
				"availableSlots": 0,
				"totalSlots":     totalSlot,
			})
			return
		}

		// If midtrans is not available, return BLU transfer info
		// Comment out the 2 lines below when Midtrans is ready
		c.JSON(http.StatusOK, gin.H{"error": "This feature is not available right now. Please transfer to the BLU account below and send proof to the number below"})
		return

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
		qrValue := fmt.Sprintf("qris-%s-%s", teamCode, now)

		// Pick registration fee using team leader user type
		var registrationFee int
		err = db.Get(&registrationFee, `
			SELECT COALESCE(
				CASE
					WHEN u.user_type = 'Binusian' THEN c.binusian_registration_fee
					ELSE c.non_binusian_registration_fee
				END,
				c.non_binusian_registration_fee
			)
			FROM competitions c
			JOIN registered_competitions rc ON c.id = rc.competition_id
			LEFT JOIN registered_competition_members rcm
				ON rcm.registered_competition_id = rc.id AND rcm.is_team_leader = true
			LEFT JOIN users u ON u.id = rcm.user_id
			WHERE rc.team_code = $1
		`, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team or competition not found"})
			return
		}

		// Calculate total amount with 0.7% fee
		totalAmount := float64(registrationFee) * 1.007

		// Create Midtrans QR
		qrResponse, err := CreateMidtransQR(db, qrValue, totalAmount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Midtrans QR"})
			return
		}

		// Update order_id, qr_url, and valid_time
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

		expiryUTC := qrResponse.ExpiryTime.Add(-7 * time.Hour)
		c.JSON(http.StatusOK, gin.H{
			"qrLink":     qrResponse.URL,
			"expiryTime": expiryUTC.Format(time.RFC3339),
		})
	}
}
