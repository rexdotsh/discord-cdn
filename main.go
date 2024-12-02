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

type DiscordClient struct {
	token  string
	client *http.Client
}

func NewDiscordClient(token string) *DiscordClient {
	return &DiscordClient{
		token:  token,
		client: &http.Client{},
	}
}

func (c *DiscordClient) RefreshAttachmentURL(attachmentURL string) (string, error) {
	body := map[string]interface{}{
		"attachment_urls": []string{attachmentURL},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://discord.com/api/v9/attachments/refresh-urls", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discord API error: %d", resp.StatusCode)
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

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	discordClient := NewDiscordClient(config.Token)
	router := gin.Default()
	router.GET("/*encodedURL", handleURL(discordClient))

	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleURL(client *DiscordClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		encodedURL := strings.TrimPrefix(c.Param("encodedURL"), "/")
		if encodedURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
			return
		}

		decodedURL, err := url.PathUnescape(encodedURL)
		if err != nil {
			log.Printf("Failed to decode URL: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
			return
		}

		parsedLink := parseLink(decodedURL)
		if parsedLink.Error != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": parsedLink.Error})
			return
		}

		attachmentURL := fmt.Sprintf("https://cdn.discordapp.com/attachments/%d/%d/%s",
			parsedLink.Data.ChannelID, parsedLink.Data.FileID, parsedLink.Data.FileName)

		newURL, err := client.RefreshAttachmentURL(attachmentURL)
		if err != nil {
			log.Printf("Error refreshing attachment URL: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to refresh URL"})
			return
		}

		c.Redirect(http.StatusMovedPermanently, newURL)
	}
}

func parseLink(input string) *ParsedLink {
	input = cleanURL(input)
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
		return &ParsedLink{Error: "File name must include extension"}
	}

	return &ParsedLink{
		Data: &LinkData{
			ChannelID: channelID,
			FileID:    fileID,
			FileName:  parts[2],
		},
	}
}

func cleanURL(url string) string {
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}
	if idx := strings.Index(url, "attachments/"); idx != -1 {
		url = url[idx+len("attachments/"):]
	}
	return url
}

func loadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// continue with environment variables
	}

	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid port value: %w", err)
	}

	token := getEnv("TOKEN", "")
	if token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	return &Config{
		Token: token,
		Port:  port,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
