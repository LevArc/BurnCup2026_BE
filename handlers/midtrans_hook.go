package handlers

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

type MidtransNotification struct {
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	PaymentType       string `json:"payment_type"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	SignatureKey      string `json:"signature_key"`
}

// MidtransWebhookHandler handles payment notifications from Midtrans
func MidtransWebhookHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var notification MidtransNotification
		if err := c.ShouldBindJSON(&notification); err != nil {
			log.Printf("Invalid webhook payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		log.Printf("Received Midtrans notification for order: %s, status: %s",
			notification.OrderID, notification.TransactionStatus)

		// Convert GrossAmount to int64
		grossAmountFloat, err := strconv.ParseFloat(notification.GrossAmount, 64)
		if err != nil {
			log.Printf("Invalid gross amount format: %s, error: %v", notification.GrossAmount, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gross amount"})
			return
		}
		grossAmountInt64 := int64(grossAmountFloat)

		// Verify signature
		if !verifySignature(notification) {
			log.Printf("Invalid signature for order: %s", notification.OrderID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}

		// Only process successful payments
		if notification.TransactionStatus != "settlement" && notification.TransactionStatus != "capture" {
			log.Printf("Ignoring non-successful payment status: %s for order: %s",
				notification.TransactionStatus, notification.OrderID)
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
			return
		}

		// Extract team code from order ID (format: qris-TeamCode-UnixTime)
		parts := strings.Split(notification.OrderID, "-")
		if len(parts) < 3 {
			log.Printf("Invalid order ID format: %s", notification.OrderID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID format"})
			return
		}
		teamCode := parts[1]

		// Start transaction
		tx, err := db.Beginx()
		if err != nil {
			log.Printf("Failed to start transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		defer tx.Rollback()

		// Lock the competition row to serialize slot checks
		var lockedCompetitionID string
		err = tx.QueryRowx(`
			SELECT id
			FROM competitions
			WHERE id = (
				SELECT competition_id
				FROM registered_competitions
				WHERE team_code = $1
			)
			FOR UPDATE
		`, teamCode).Scan(&lockedCompetitionID)
		if err != nil {
			log.Printf("Failed to lock competition row for team %s: %v", teamCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// Get team registration details
		var teamInfo struct {
			TeamID        string `db:"team_id"`
			CompetitionID string `db:"competition_id"`
			IsPaid        bool   `db:"is_paid"`
		}
		err = tx.Get(&teamInfo, `
			SELECT rc.id as team_id, rc.competition_id, rc.is_paid
			FROM registered_competitions rc
			WHERE rc.team_code = $1
		`, teamCode)
		if err != nil {
			log.Printf("Team not found for code: %s, error: %v", teamCode, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Check if already paid
		if teamInfo.IsPaid {
			log.Printf("Team %s is already paid", teamCode)
			c.JSON(http.StatusOK, gin.H{"status": "already_paid"})
			return
		}

		// Get team leader user type
		var leaderUserType string
		err = tx.QueryRowx(`
			SELECT COALESCE(u.user_type, '')
			FROM users u
			JOIN registered_competition_members rcm ON u.id = rcm.user_id
			WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = true
			LIMIT 1
		`, teamInfo.TeamID).Scan(&leaderUserType)
		if err != nil {
			log.Printf("Failed to get leader user type for team %s: %v", teamCode, err)
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
		err = tx.QueryRowx(
			fmt.Sprintf(`SELECT %s FROM competition_slots WHERE competition_id = $1`, slotColumn),
			teamInfo.CompetitionID,
		).Scan(&totalSlot)
		if err != nil {
			log.Printf("Failed to get slot info for competition %s: %v", teamInfo.CompetitionID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Slot info not found for this competition"})
			return
		}

		// Count paid teams in same category
		categoryFilter := "LOWER(u.user_type) != 'binusian'"
		if strings.EqualFold(leaderUserType, "Binusian") {
			categoryFilter = "LOWER(u.user_type) = 'binusian'"
		}
		var paidTeams int
		err = tx.Get(&paidTeams, fmt.Sprintf(`
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
			log.Printf("Failed to count paid teams: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		log.Printf("Competition has %d paid teams out of %d slots for category: %s",
			paidTeams, totalSlot, leaderUserType)

		// Check if slot is full — issue refund if so
		if paidTeams >= totalSlot {
			log.Printf("Competition is full for category %s. Issuing refund for team: %s", leaderUserType, teamCode)
			err = issueRefund(notification.TransactionID, grossAmountInt64)
			if err != nil {
				log.Printf("Failed to issue refund for team %s: %v", teamCode, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue refund"})
				return
			}
			_, err = tx.Exec(`
				UPDATE registered_competitions
				SET updated_at = NOW()
				WHERE team_code = $1
			`, teamCode)
			if err != nil {
				log.Printf("Failed to update team after refund: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}
			if err = tx.Commit(); err != nil {
				log.Printf("Failed to commit refund transaction: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}
			log.Printf("Refund issued successfully for team: %s", teamCode)
			c.JSON(http.StatusOK, gin.H{
				"status":  "refunded",
				"message": "Competition slot is full for your category, refund has been issued",
			})
			return
		}

		// Slot available — mark team as paid
		_, err = tx.Exec(`
			UPDATE registered_competitions
			SET is_paid = true, updated_at = NOW()
			WHERE team_code = $1
		`, teamCode)
		if err != nil {
			log.Printf("Failed to mark team as paid: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if err = tx.Commit(); err != nil {
			log.Printf("Failed to commit payment transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		log.Printf("Payment processed successfully for team: %s", teamCode)
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Payment processed successfully",
		})
	}
}

// verifySignature verifies the Midtrans signature
func verifySignature(notification MidtransNotification) bool {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		log.Printf("MIDTRANS_SERVER_KEY not set")
		return false
	}
	signatureString := notification.OrderID + notification.StatusCode +
		notification.GrossAmount + serverKey
	hash := sha512.Sum512([]byte(signatureString))
	signature := hex.EncodeToString(hash[:])
	return signature == notification.SignatureKey
}

// issueRefund issues a refund through Midtrans
func issueRefund(transactionID string, amount int64) error {
	c := coreapi.Client{}
	c.New(os.Getenv("MIDTRANS_SERVER_KEY"), midtrans.Sandbox)
	refundReq := &coreapi.RefundReq{
		Amount: amount,
		Reason: "Competition slot is full for your category",
	}
	_, err := c.RefundTransaction(transactionID, refundReq)
	if err != nil {
		return fmt.Errorf("midtrans refund failed: %w", err)
	}
	log.Printf("Refund issued for transaction: %s, amount: %d", transactionID, amount)
	return nil
}
