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

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

func main() {
	// // Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env file")
	}

	// Example DSN: "host=localhost port=5432 user=postgres password=yourpassword dbname=yourdb sslmode=disable"
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN environment variable is not set")
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
		
	// Hard limit to prevent exceeding Azure B1ms 35-connection max
    db.SetMaxOpenConns(25)
    // Keep connections open in the background to prevent latency from reconnecting
    db.SetMaxIdleConns(25)
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

		// Public endpoints
		api.GET("/competitions", handlers.GetCompetitionsHandler(db))

		api.GET("/competitions/:id", handlers.GetCompetitionByIDHandler(db))
		api.GET("/get-remaining-team-slot/:competitionId", handlers.GetRemainingTeamSlotHandler(db))
		api.POST("/midtrans/hook", handlers.MidtransWebhookHandler(db))
		api.GET("/qr/:value", handlers.GenerateQRHandler())
		api.GET("/ping-is-paid-team-slot/:teamId", handlers.PingIsPaidTeamSlotHandler(db))

		// Auth Group
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.RegisterHandler(db))
			auth.POST("/login", handlers.LoginHandler(db))
			auth.GET("/google", handlers.GoogleLoginHandler())
			auth.GET("/google/callback", handlers.GoogleCallbackHandler(db))
		}

		// Protected Group (Standard Users)
		protected := api.Group("/protected")
		protected.Use(middleware.JWTAuthMiddleware())
		{
			protected.GET("", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "You are authorized to access this protected endpoint"})
			})
			protected.GET("/get-current-user", handlers.GetCurrentUserHandler(db))
			protected.POST("/create-update-user-profile", handlers.CreateUserProfileHandler(db))
			protected.POST("/create-team", handlers.CreateTeamHandler(db))
			protected.GET("/get-teams", handlers.GetUserCompetitionsHandler(db))
			protected.POST("/join-team", handlers.JoinTeamHandler(db))
			protected.DELETE("/delete-team-member", handlers.DeleteTeamMemberHandler(db))
			protected.GET("/get-qr-link/:teamCode", handlers.GetQRLinkHandler(db))
		}

		// Admin Group (Only Accessible by Admins in the List)
		admin := api.Group("/admin")
		admin.Use(middleware.AdminAuthMiddleware())
		{
			// The routes here are now accessible via /api/admin/basic-info, etc.
			admin.GET("/basic-info", handlers.GetAdminBasicInfoHandler(db))
			admin.GET("/competitions-statistics", handlers.GetAdminCompetitionStatisticHandler(db))
			admin.GET("/all-teams", handlers.GetAllTeamsHandler(db))
			admin.POST("/add-competition", handlers.AddCompetitionHandler(db))
			admin.POST("/update-competition/:id", handlers.UpdateCompetitionHandler(db))
			admin.DELETE("/delete-competition/:id", handlers.DeleteCompetitionHandler(db))
			admin.DELETE("/delete-team/:teamCode", handlers.DeleteTeamHandler(db))
		}
	}

	// CORS Configuration
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:5173", 
			"http://host.docker.internal:3000", 
			"https://burncuptesting.notchgnas.com", 
			"http://localhost:3001", 
			"https://burncup.notchgnas.com", 
			"https://burncup-fe-341997010337.asia-southeast1.run.app", 
			"https://burncup-backend-341997010337.asia-southeast1.run.app", 
			"https://burncup.com",
		},
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