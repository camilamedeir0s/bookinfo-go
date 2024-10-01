package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type Review struct {
	Reviewer string  `json:"reviewer"`
	Text     string  `json:"text"`
	Rating   *Rating `json:"rating,omitempty"`
}

type Rating struct {
	Stars int    `json:"stars"`
	Color string `json:"color"`
}

type Response struct {
	ID          string   `json:"id"`
	PodName     string   `json:"podname"`
	ClusterName string   `json:"clustername"`
	Reviews     []Review `json:"reviews"`
}

var (
	// ratingsEnabled = getEnvAsBool("ENABLE_RATINGS", false)
	starColor = getEnv("STAR_COLOR", "black")
	// ratingsService = fmt.Sprintf("http://%s:%s/ratings",
	// 	getEnv("RATINGS_HOSTNAME", "ratings"),
	// 	getEnv("RATINGS_SERVICE_PORT", "8080"),
	// )

	ratingsEnabled = getEnvAsBool("ENABLE_RATINGS", true)
	ratingsService = "http://localhost:8085/ratings"

	podHostname = getEnv("HOSTNAME", "unknown")
	clusterName = getEnv("CLUSTER_NAME", "unknown")
	httpClient  = &http.Client{Timeout: 10 * time.Second}
)

func main() {
	router := gin.Default()

	router.GET("/health", health)
	router.GET("/reviews/:productId", bookReviewsByID)

	router.Run(":9086")
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "Reviews is healthy"})
}

func bookReviewsByID(c *gin.Context) {
	productId := c.Param("productId")
	starsReviewer1 := -1
	starsReviewer2 := -1

	if ratingsEnabled {
		ratingsResponse, err := getRatings(productId, c.Request)
		fmt.Println(ratingsResponse)
		if err == nil {
			if ratings, exists := ratingsResponse["ratings"].(map[string]interface{}); exists {
				if reviewer1, exists := ratings["Reviewer1"].(float64); exists {
					starsReviewer1 = int(reviewer1)
				}
				if reviewer2, exists := ratings["Reviewer2"].(float64); exists {
					starsReviewer2 = int(reviewer2)
				}
			}
		}
	}

	response := getJsonResponse(productId, starsReviewer1, starsReviewer2)
	c.JSON(http.StatusOK, response)
}

func getRatings(productId string, req *http.Request) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", ratingsService, productId)
	request, _ := http.NewRequest("GET", url, nil)
	fmt.Println(url)

	for _, header := range headersToPropagate {
		if value := req.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}

	resp, err := httpClient.Do(request)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func getJsonResponse(productId string, starsReviewer1, starsReviewer2 int) Response {
	reviews := []Review{
		{
			Reviewer: "Reviewer1",
			Text:     "An extremely entertaining play by Shakespeare. The slapstick humour is refreshing!",
		},
		{
			Reviewer: "Reviewer2",
			Text:     "Absolutely fun and entertaining. The play lacks thematic depth when compared to other plays by Shakespeare.",
		},
	}

	if ratingsEnabled {
		if starsReviewer1 != -1 {
			reviews[0].Rating = &Rating{Stars: starsReviewer1, Color: starColor}
		} else {
			reviews[0].Rating = &Rating{Stars: -1, Color: "Ratings service is unavailable"}
		}

		if starsReviewer2 != -1 {
			reviews[1].Rating = &Rating{Stars: starsReviewer2, Color: starColor}
		} else {
			reviews[1].Rating = &Rating{Stars: -1, Color: "Ratings service is unavailable"}
		}
	}

	return Response{
		ID:          productId,
		PodName:     podHostname,
		ClusterName: clusterName,
		Reviews:     reviews,
	}
}

// Utility functions

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true"
	}
	return fallback
}

var headersToPropagate = []string{
	"x-request-id",
	"x-ot-span-context",
	"x-datadog-trace-id",
	"x-datadog-parent-id",
	"x-datadog-sampling-priority",
	"traceparent",
	"tracestate",
	"x-cloud-trace-context",
	"grpc-trace-bin",
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",
	"sw8",
	"end-user",
	"user-agent",
	"cookie",
	"authorization",
	"jwt",
}
