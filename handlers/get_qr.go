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
		now := strconv.FormatInt(time.Now().Unix(), 10)
		qrValue := fmt.Sprintf("qr-%s-%s", teamCode, now)

		// Update order_id in registered_competitions
		_, err := db.Exec(
			`UPDATE registered_competitions SET order_id = $1 WHERE team_code = $2`,
			qrValue, teamCode,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order_id"})
			return
		}

		// Get the request's scheme and host to build the full URL
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host
		fullLink := fmt.Sprintf("%s://%s/api/qr/%s", scheme, host, qrValue)

		c.JSON(http.StatusOK, gin.H{"qrLink": fullLink})
	}
}
