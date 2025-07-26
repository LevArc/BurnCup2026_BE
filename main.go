package main

import (
	_ "embed"
	"log"
	"net/http"
	"os"

	"github.com/NotchG/BurnCup/handlers"
	"github.com/NotchG/BurnCup/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	// "github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

func main() {
	// // Load environment variables from .env file
	// if err := godotenv.Load(); err != nil {
	// 	log.Println("No .env file found or error loading .env file")
	// }

	// Example DSN: "host=localhost port=5432 user=postgres password=yourpassword dbname=yourdb sslmode=disable"
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN environment variable is not set")
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.String(200, "BurnCup API is running")
	})

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "Not found"})
	})

	api := r.Group("/api")
	{
		api.GET("/public", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "This is a public endpoint"})
		})

		// GET /api/competitions - public endpoint to list competitions
		api.GET("/competitions", handlers.GetCompetitionsHandler(db))

		api.GET("/competitions/:id", handlers.GetCompetitionByIDHandler(db))
		api.GET("/get-remaining-team-slot/:competitionId", handlers.GetRemainingTeamSlotHandler(db))

		api.POST("/midtrans/hook", handlers.MidtransWebhookHandler(db))

		protected := api.Group("/protected")
		protected.Use(middleware.JWTAuthMiddleware())
		protected.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "You are authorized to access this protected endpoint"})
		})

		protected.GET("/get-current-user", handlers.GetCurrentUserHandler(db))
		protected.POST("/create-update-user-profile", handlers.CreateUserProfileHandler(db))

		protected.POST("/create-team", handlers.CreateTeamHandler(db))
		protected.GET("/get-teams", handlers.GetUserCompetitionsHandler(db))
		protected.POST("/join-team", handlers.JoinTeamHandler(db))
		protected.DELETE("/delete-team-member", handlers.DeleteTeamMemberHandler(db))
		api.GET("/ping-is-paid-team-slot/:teamId", handlers.PingIsPaidTeamSlotHandler(db))

		api.GET("/qr/:value", handlers.GenerateQRHandler())
		protected.GET("/get-qr-link/:teamCode", handlers.GetQRLinkHandler(db))

		protected.GET("/admin-basic-info", handlers.GetAdminBasicInfoHandler(db))
		protected.GET("/admin-competitions-statistics", handlers.GetAdminCompetitionStatisticHandler(db))
		protected.GET("/admin-all-teams", handlers.GetAllTeamsHandler(db))
		protected.POST("/admin-add-competition", handlers.AddCompetitionHandler(db))
		protected.POST("/admin-update-competition/:id", handlers.UpdateCompetitionHandler(db))
		protected.DELETE("/admin-delete-competition/:id", handlers.DeleteCompetitionHandler(db))
		protected.DELETE("/admin-delete-team/:teamCode", handlers.DeleteTeamHandler(db))
	}

	// Use rs/cors to wrap the Gin engine
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://host.docker.internal:3000", "https://burncuptesting.notchgnas.com", "http://localhost:3001", "https://burncup.notchgnas.com", "https://burncup-fe-341997010337.asia-southeast1.run.app", "https://burncup-backend-341997010337.asia-southeast1.run.app", "https://burncup.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
	})

	log.Println("Server is running on port 8080")
	if err := http.ListenAndServe("0.0.0.0:8080", corsHandler.Handler(r)); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
