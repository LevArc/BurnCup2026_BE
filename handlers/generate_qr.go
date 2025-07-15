package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/skip2/go-qrcode"
)

// GenerateQRHandler generates a QR code PNG for the given 'value' param
func GenerateQRHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        value := c.Param("value")
        if value == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing value parameter"})
            return
        }
        png, err := qrcode.Encode(value, qrcode.Medium, 256)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
            return
        }
        c.Header("Content-Type", "image/png")
        c.Writer.Write(png)
    }
}