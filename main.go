package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Config struct {
	Token string
	Port  int
}

type LinkData struct {
	ChannelID int64  `json:"channelID"`
	FileID    int64  `json:"fileID"`
	FileName  string `json:"fileName"`
}

type ParsedLink struct {
	Error string    `json:"error"`
	Data  *LinkData `json:"data"`
}

type RefreshURLsResponse struct {
	RefreshedURLs []struct {
		Original  string `json:"original"`
		Refreshed string `json:"refreshed"`
	} `json:"refreshed_urls"`
}

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	router := setupRouter(config)
	log.Printf("Server is running on port %d", config.Port)
	if err := router.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func setupRouter(config *Config) *gin.Engine {
	router := gin.Default()
	router.GET("/*encodedURL", handleURL(config))
	return router
}

func handleURL(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		encodedURL := strings.TrimPrefix(c.Param("encodedURL"), "/")
		decodedURL, err := decodeURL(encodedURL)
		if err != nil {
			log.Printf("Failed to decode URL: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
			return
		}

		if decodedURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
			return
		}

		newURL, err := fetchLatestLink(decodedURL, config.Token)
		if err != nil {
			log.Printf("Error fetching updated link: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch updated URL"})
			return
		}

		c.Redirect(http.StatusMovedPermanently, newURL)
	}
}

func fetchLatestLink(oldLink, token string) (string, error) {
	link := parseLink(oldLink)
	if link.Error != "" {
		return "", fmt.Errorf(link.Error)
	}

	attachmentURL := fmt.Sprintf("https://cdn.discordapp.com/attachments/%d/%d/%s",
		link.Data.ChannelID, link.Data.FileID, link.Data.FileName)

	requestBody := map[string]interface{}{
		"attachment_urls": []string{attachmentURL},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://discord.com/api/v9/attachments/refresh-urls", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var refreshResponse RefreshURLsResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(refreshResponse.RefreshedURLs) == 0 {
		return "", fmt.Errorf("no refreshed URLs returned")
	}

	return refreshResponse.RefreshedURLs[0].Refreshed, nil
}

func parseLink(input string) *ParsedLink {
	if strings.Contains(input, "?") {
		input = strings.Split(input, "?")[0]
	}
	if strings.Contains(input, "attachments/") {
		input = strings.Split(input, "attachments/")[1]
	}

	parts := strings.Split(input, "/")
	if len(parts) != 3 {
		return &ParsedLink{Error: "Invalid link format"}
	}

	channelID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return &ParsedLink{Error: "Invalid Channel ID"}
	}

	fileID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return &ParsedLink{Error: "Invalid File ID"}
	}

	if !strings.Contains(parts[2], ".") {
		return &ParsedLink{Error: "File name must include a file extension"}
	}

	return &ParsedLink{
		Data: &LinkData{
			ChannelID: channelID,
			FileID:    fileID,
			FileName:  parts[2],
		},
	}
}

func getConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables.")
	}

	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid port value: %w", err)
	}

	return &Config{
		Token: getEnv("TOKEN", ""),
		Port:  port,
	}, nil
}

func decodeURL(encodedURL string) (string, error) {
	return url.PathUnescape(encodedURL)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
