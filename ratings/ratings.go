package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/context"
)

var (
	userAddedRatings = make(map[int]map[string]int) // in-memory ratings
	unavailable      = false
	healthy          = true
	db               *sql.DB
	mongoClient      *mongo.Client
)

func init() {
	if os.Getenv("SERVICE_VERSION") == "v-unavailable" {
		// make the service unavailable once in 60 seconds
		go func() {
			for {
				unavailable = !unavailable
				time.Sleep(60 * time.Second)
			}
		}()
	}

	if os.Getenv("SERVICE_VERSION") == "v-unhealthy" {
		// make the service unhealthy every 15 minutes
		go func() {
			for {
				healthy = !healthy
				unavailable = !unavailable
				time.Sleep(15 * time.Minute)
			}
		}()
	}
}

func main() {
	r := gin.Default()

	// Establish database connection based on version
	if os.Getenv("SERVICE_VERSION") == "v2" {
		dbType := os.Getenv("DB_TYPE")
		if dbType == "mysql" {
			var err error
			host := os.Getenv("MYSQL_DB_HOST")
			port := os.Getenv("MYSQL_DB_PORT")
			user := os.Getenv("MYSQL_DB_USER")
			password := os.Getenv("MYSQL_DB_PASSWORD")
			dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/ratingsdb", user, password, host, port)

			db, err = sql.Open("mysql", dsn)
			if err != nil {
				log.Fatal("Could not connect to MySQL database:", err)
			}

		}
	}

	// Routes
	r.GET("/ratings/:productId", getRatings)
	r.POST("/ratings/:productId", postRatings)
	r.GET("/health", healthCheck)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

func getRatings(c *gin.Context) {
	if os.Getenv("SERVICE_VERSION") == "v-unavailable" || os.Getenv("SERVICE_VERSION") == "v-unhealthy" {
		if unavailable {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
			return
		}
	}

	productId, err := strconv.Atoi(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide numeric product ID"})
		return
	}

	if os.Getenv("SERVICE_VERSION") == "v2" {

		var firstRating, secondRating int

		if os.Getenv("DB_TYPE") == "mysql" {
			err = db.Ping()
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"error": "could not connect to ratings database"})
				return
			}

			rows, err := db.Query("SELECT Rating FROM ratings LIMIT 2")
			if err != nil {
				fmt.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not perform select"})
				return
			}
			defer rows.Close()

			count := 0
			for rows.Next() {
				if count == 0 {
					err = rows.Scan(&firstRating)
				} else if count == 1 {
					err = rows.Scan(&secondRating)
				}
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve ratings"})
					return
				}
				count++
			}

			if count == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "ratings not found"})
				return
			}

		} else {
			var err error
			mongoURL := os.Getenv("MONGO_DB_URL")

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Substitui mongoClient.Connect por mongo.Connect
			mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
			if err != nil {
				log.Fatal("Could not connect to MongoDB:", err)
			}

			collection := mongoClient.Database("test").Collection("ratings")
			cursor, err := collection.Find(ctx, bson.M{})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not connect to ratings database"})
				return
			}
			defer cursor.Close(ctx)

			var ratingsData []bson.M // Usando bson.M para mapear os dados diretamente
			if err = cursor.All(ctx, &ratingsData); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not parse ratings data"})
				return
			}

			firstRating, secondRating := 0, 0

			// Verifica se existem registros suficientes e atribui os ratings
			if len(ratingsData) > 0 {
				if val, ok := ratingsData[0]["rating"].(int32); ok {
					firstRating = int(val)
				} else {
					fmt.Println("Rating for Reviewer1 not found or not an integer")
				}
			}

			if len(ratingsData) > 1 {
				if val, ok := ratingsData[1]["rating"].(int32); ok {
					secondRating = int(val)
				} else {
					fmt.Println("Rating for Reviewer2 not found or not an integer")
				}
			}

			result := map[string]interface{}{
				"id": productId,
				"ratings": map[string]int{
					"Reviewer1": firstRating,
					"Reviewer2": secondRating,
				},
			}

			c.JSON(http.StatusOK, result)

		}

	} else {
		c.JSON(http.StatusOK, getLocalReviews(productId))
	}

}

func postRatings(c *gin.Context) {
	productId, err := strconv.Atoi(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide numeric product ID"})
		return
	}

	var ratings map[string]int
	if err := c.ShouldBindJSON(&ratings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide valid ratings JSON"})
		return
	}

	if os.Getenv("SERVICE_VERSION") == "v2" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Post not implemented for database backed ratings"})
	} else {
		c.JSON(http.StatusOK, putLocalReviews(productId, ratings))
	}
}

func healthCheck(c *gin.Context) {
	if healthy {
		c.JSON(http.StatusOK, gin.H{"status": "Ratings is healthy"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Ratings is not healthy"})
	}
}

func putLocalReviews(productId int, ratings map[string]int) map[string]interface{} {
	userAddedRatings[productId] = ratings
	return getLocalReviews(productId)
}

func getLocalReviews(productId int) map[string]interface{} {
	if val, ok := userAddedRatings[productId]; ok {
		return map[string]interface{}{"id": productId, "ratings": val}
	}

	return map[string]interface{}{
		"id": productId,
		"ratings": map[string]int{
			"Reviewer1": 5,
			"Reviewer2": 4,
		},
	}
}
