package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NotchG/BurnCup/handlers"
	"github.com/NotchG/BurnCup/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

func execSQLFile(db *sqlx.DB, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = db.Exec(string(sqlBytes))
	return err
}

func main() {
	// Example DSN: "host=localhost port=5432 user=postgres password=yourpassword dbname=yourdb sslmode=disable"
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=postgres port=5432 user=postgres password=postgres dbname=burncup_dev sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Run TABLES.sql and INSERT.sql on startup
	if err := execSQLFile(db, "TABLES.sql"); err != nil {
		log.Fatalf("Failed to execute TABLES.sql: %v", err)
	}
	if err := execSQLFile(db, "INSERT.sql"); err != nil {
		log.Fatalf("Failed to execute INSERT.sql: %v", err)
	}

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

		protected := api.Group("/protected")
		protected.Use(middleware.JWTAuthMiddleware())
		protected.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "You are authorized to access this protected endpoint"})
		})

		protected.GET("/get-current-user", handlers.GetCurrentUserHandler(db))
		protected.POST("/create-update-user-profile", handlers.CreateUserProfileHandler(db))

		protected.POST("/create-team", handlers.CreateTeamHandler(db))
		protected.GET("/get-teams", handlers.GetUserCompetitionsHandler(db))
	}

	// Use rs/cors to wrap the Gin engine
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://host.docker.internal:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
	})

	log.Println("Server is running on port 8000")
	if err := http.ListenAndServe("0.0.0.0:8000", corsHandler.Handler(r)); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
