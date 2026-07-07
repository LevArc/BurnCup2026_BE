package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

type QRResponse struct {
	URL        string    `json:"url"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func CreateMidtransQR(db *sqlx.DB, orderID string, amount float64) (*QRResponse, error) {
	log.Printf("CreateMidtransQR called with orderID: %s, amount: %.2f", orderID, amount)

	midtransEnvironment := midtrans.Production
	if strings.EqualFold(os.Getenv("MIDTRANS_ENV"), "sandbox") {
		midtransEnvironment = midtrans.Sandbox
	}

	// Parse team code from orderID (format: qris-TeamCode-UnixTime)
	parts := strings.Split(orderID, "-")
	if len(parts) < 3 {
		log.Printf("Invalid orderID format: %s", orderID)
		return nil, fmt.Errorf("invalid orderID format, expected: qris-TeamCode-UnixTime")
	}
	teamCode := parts[1]
	log.Printf("Extracted team code: %s", teamCode)

	// Get team leader details and competition info
	teamLeader, competition, err := getTeamLeaderAndCompetition(db, teamCode)
	if err != nil {
		log.Printf("Failed to get team details: %v", err)
		return nil, fmt.Errorf("failed to get team details: %w", err)
	}

	// Validate required fields
	if teamLeader.FullName == "" || teamLeader.Email == "" {
		log.Printf("Team leader information incomplete: %+v", teamLeader)
		return nil, fmt.Errorf("team leader information incomplete")
	}

	// Initialize client
	c := coreapi.Client{}
	c.New(os.Getenv("MIDTRANS_SERVER_KEY"), midtransEnvironment)

	if notificationURL := os.Getenv("MIDTRANS_NOTIFICATION_URL"); notificationURL != "" {
		c.Options.SetPaymentOverrideNotification(notificationURL)
		log.Printf("Using Midtrans notification URL: %s", notificationURL)
	}

	// Ensure phone number format is correct for Indonesia
	phone := teamLeader.PhoneNumber
	if !strings.HasPrefix(phone, "+62") && !strings.HasPrefix(phone, "62") {
		if strings.HasPrefix(phone, "0") {
			phone = "+62" + phone[1:]
		} else {
			phone = "+62" + phone
		}
	}
	log.Printf("Formatted phone number: %s", phone)

	// Prepare charge request with team leader as customer and competition as item
	req := &coreapi.ChargeReq{
		PaymentType: coreapi.PaymentTypeQris,
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(amount),
		},
		CustomerDetails: &midtrans.CustomerDetails{
			FName: teamLeader.FullName,
			Email: teamLeader.Email,
			Phone: phone,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    competition.ID,
				Price: int64(amount),
				Qty:   1,
				Name:  fmt.Sprintf("Registration Fee - %s", competition.Name),
			},
		},
	}

	log.Printf("Sending charge request to Midtrans for orderID: %s", orderID)

	// Send charge request
	resp, err := c.ChargeTransaction(req)

	// Handle Midtrans specific error type
	if err != nil {
		if midtransErr, ok := err.(*midtrans.Error); ok {
			if midtransErr == nil || (midtransErr.RawError == nil && midtransErr.StatusCode == 0) {
				log.Printf("Midtrans returned empty error, treating as success")
			} else {
				log.Printf("Midtrans charge failed: StatusCode=%d, Message=%s", midtransErr.StatusCode, midtransErr.Message)
				return nil, fmt.Errorf("failed to create charge: status=%d, message=%s", midtransErr.StatusCode, midtransErr.Message)
			}
		} else {
			log.Printf("Midtrans charge failed: %v", err)
			return nil, fmt.Errorf("failed to create charge: %w", err)
		}
	}

	log.Printf("SUCCESS: No error from ChargeTransaction")

	// Check if response is nil
	if resp == nil {
		log.Printf("Midtrans returned nil response")
		return nil, fmt.Errorf("midtrans returned empty response")
	}

	log.Printf("Midtrans response status: %s, message: %s", resp.StatusCode, resp.StatusMessage)

	// Check response status
	if resp.StatusCode != "201" {
		log.Printf("Midtrans error response: %s - %s", resp.StatusCode, resp.StatusMessage)
		return nil, fmt.Errorf("midtrans error: %s - %s", resp.StatusCode, resp.StatusMessage)
	}

	// Parse expiry time
	expiryTime, err := time.Parse("2006-01-02 15:04:05", resp.ExpiryTime)
	if err != nil {
		log.Printf("Failed to parse expiry time: %v", err)
		// Set a default expiry time (15 minutes from now) if parsing fails
		expiryTime = time.Now().Add(15 * time.Minute)
	}

	// Extract QR URL from actions
	for _, action := range resp.Actions {
		if action.Name == "generate-qr-code" {
			log.Printf("QR URL generated successfully: %s, expires at: %s", action.URL, expiryTime.Format(time.RFC3339))
			return &QRResponse{
				URL:        action.URL,
				ExpiryTime: expiryTime,
			}, nil
		}
	}

	log.Printf("QR URL not found in Midtrans response actions")
	return nil, fmt.Errorf("QR URL not found in Midtrans response")
}

type TeamLeader struct {
	Email       string
	FullName    string
	PhoneNumber string
}

type Competition struct {
	ID   string
	Name string
}

func getTeamLeaderAndCompetition(db *sqlx.DB, teamCode string) (*TeamLeader, *Competition, error) {
	log.Printf("Querying database for team code: %s", teamCode)

	query := `
        SELECT 
            u.email,
            u.full_name,
            u.phone_number,
            c.id::text,
            c.name
        FROM registered_competitions rc
        JOIN registered_competition_members rcm ON rc.id = rcm.registered_competition_id
        JOIN users u ON rcm.user_email = u.email
        JOIN competitions c ON rc.competition_id = c.id
        WHERE rc.team_code = $1 AND rcm.is_team_leader = true
    `

	var teamLeader TeamLeader
	var competition Competition

	err := db.QueryRowx(query, teamCode).Scan(
		&teamLeader.Email,
		&teamLeader.FullName,
		&teamLeader.PhoneNumber,
		&competition.ID,
		&competition.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No team found for team code: %s", teamCode)
			return nil, nil, fmt.Errorf("team not found or no team leader assigned for team code: %s", teamCode)
		}
		log.Printf("Database query error for team code %s: %v", teamCode, err)
		return nil, nil, fmt.Errorf("database query error: %w", err)
	}

	log.Printf("Team found - TeamCode: %s, TeamLeader: %+v, Competition: %+v", teamCode, teamLeader, competition)

	return &teamLeader, &competition, nil
}
