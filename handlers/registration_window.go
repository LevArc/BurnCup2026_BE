package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

// A DATE is a calendar day, not an instant. Registration is inclusive of
// both dates in Western Indonesian Time (UTC+7), regardless of server timezone.
var wib = time.FixedZone("WIB", 7*60*60)

func registrationWindowError(start, end string, now time.Time) string {
	today := now.In(wib).Format("2006-01-02")
	if today < start {
		return "Registration has not started"
	}
	if today > end {
		return "Registration has ended"
	}
	return ""
}

// checkRegistrationOpen returns an HTTP status and message for a rejected request.
// An open registration returns status 0 and an empty message.
func checkRegistrationOpen(db *sqlx.DB, competitionID string) (int, string) {
	var start, end string
	err := db.QueryRowx(`SELECT registration_start_date::text, registration_end_date::text FROM competitions WHERE id = $1`, competitionID).Scan(&start, &end)
	if err == sql.ErrNoRows {
		return http.StatusNotFound, "Competition not found"
	}
	if err != nil {
		return http.StatusInternalServerError, "Failed to check registration period"
	}
	if message := registrationWindowError(start, end, time.Now()); message != "" {
		return http.StatusForbidden, message
	}
	return 0, ""
}
