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
		grossAmountInt, err := strconv.ParseFloat(notification.GrossAmount, 64)
		if err != nil {
			log.Printf("Invalid gross amount format: %s, error: %v", notification.GrossAmount, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gross amount"})
			return
		}
		grossAmountInt64 := int64(grossAmountInt)

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

		// Extract team code from order ID (format: qr-TeamCode-UnixTime)
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

		// Get team registration details
		var teamInfo struct {
			CompetitionID string `db:"competition_id"`
			TeamSlot      int    `db:"team_slot"`
			IsPaid        bool   `db:"is_paid"`
		}

		err = tx.Get(&teamInfo, `
            SELECT rc.competition_id, c.team_slot, rc.is_paid
            FROM registered_competitions rc
            JOIN competitions c ON rc.competition_id = c.id
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

		// Count current paid teams for this competition
		var paidTeams int
		err = tx.Get(&paidTeams, `
            SELECT COUNT(*) 
            FROM registered_competitions 
            WHERE competition_id = $1 AND is_paid = true
        `, teamInfo.CompetitionID)
		if err != nil {
			log.Printf("Failed to count paid teams: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		log.Printf("Competition has %d paid teams out of %d slots", paidTeams, teamInfo.TeamSlot)

		// Check if team slot is full
		if paidTeams >= teamInfo.TeamSlot {
			log.Printf("Competition is full. Issuing refund for team: %s", teamCode)

			// Issue refund
			err = issueRefund(notification.TransactionID, grossAmountInt64)
			if err != nil {
				log.Printf("Failed to issue refund for team %s: %v", teamCode, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue refund"})
				return
			}

			// Update team status to indicate refund
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
				"message": "Competition is full, refund has been issued",
			})
			return
		}

		// Team slot available, mark as paid
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

	// Create signature string: order_id+status_code+gross_amount+server_key
	signatureString := notification.OrderID + notification.StatusCode +
		notification.GrossAmount + serverKey

	// Create SHA512 hash
	hash := sha512.Sum512([]byte(signatureString))
	signature := hex.EncodeToString(hash[:])

	return signature == notification.SignatureKey
}

// issueRefund issues a refund through Midtrans
func issueRefund(transactionID string, amount int64) error {
	// Initialize Midtrans client
	c := coreapi.Client{}
	c.New(os.Getenv("MIDTRANS_SERVER_KEY"), midtrans.Sandbox)

	// Create refund request
	refundReq := &coreapi.RefundReq{
		Amount: amount,
		Reason: "Competition slot is full",
	}

	// Issue refund
	_, err := c.RefundTransaction(transactionID, refundReq)
	if err != nil {
		return fmt.Errorf("midtrans refund failed: %w", err)
	}

	log.Printf("Refund issued for transaction: %s, amount: %d", transactionID, amount)
	return nil
}
