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
			initMySQL()
		} else {
			initMongoDB()
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

func initMySQL() {
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

func initMongoDB() {
	var err error
	mongoURL := os.Getenv("MONGO_DB_URL")
	mongoClient, err = mongo.NewClient(options.Client().ApplyURI(mongoURL))
	if err != nil {
		log.Fatal("Could not create MongoDB client:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = mongoClient.Connect(ctx)
	if err != nil {
		log.Fatal("Could not connect to MongoDB:", err)
	}
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

		err = db.Ping()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"error": "could not connect to ratings database"})
			return
		}

		if os.Getenv("DB_TYPE") == "mysql" {
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
			collection := mongoClient.Database("test").Collection("ratings")
			filter := bson.M{"id": productId}
			var result []bson.M
			cursor, err := collection.Find(context.TODO(), filter)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load ratings from database"})
				return
			}
			if err = cursor.All(context.TODO(), &result); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not decode ratings"})
				return
			}

			if len(result) > 0 {
				firstRating = result[0]["rating"].(int)
			}
			if len(result) > 1 {
				secondRating = result[1]["rating"].(int)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"id": productId,
			"ratings": gin.H{
				"Reviewer1": firstRating,
				"Reviewer2": secondRating,
			},
		})
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
