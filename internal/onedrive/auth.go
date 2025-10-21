package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/kpauljoseph/notesankify/pkg/logger"
)

const (
	// Default scopes for OneDrive access
	defaultScope = "https://graph.microsoft.com/.default"

	// Token file name
	tokenFileName = "onedrive_token.json"
)

// Authenticator handles OneDrive authentication
type Authenticator struct {
	config    *Config
	logger    *logger.Logger
	tokenPath string
	cred      azcore.TokenCredential
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(config *Config, log *logger.Logger) (*Authenticator, error) {
	if config.ClientID == "" {
		return nil, fmt.Errorf("client_id is required for OneDrive authentication")
	}

	// Use common tenant if not specified
	tenantID := config.TenantID
	if tenantID == "" {
		tenantID = "common"
	}

	// Get user home directory for token storage
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	tokenPath := filepath.Join(homeDir, ".notesankify", tokenFileName)

	auth := &Authenticator{
		config:    config,
		logger:    log,
		tokenPath: tokenPath,
	}

	return auth, nil
}

// Authenticate performs the authentication flow
func (a *Authenticator) Authenticate(ctx context.Context) error {
	// First, try to load existing token
	if token, err := a.loadToken(); err == nil && !a.isTokenExpired(token) {
		a.logger.Debug("Using cached authentication token")
		// Token is valid, create credential from stored token
		// Note: For production, implement refresh token flow
		return nil
	}

	a.logger.Info("Starting OneDrive authentication flow...")

	// Use device code flow for CLI authentication
	tenantID := a.config.TenantID
	if tenantID == "" {
		tenantID = "common"
	}

	options := &azidentity.DeviceCodeCredentialOptions{
		TenantID: tenantID,
		ClientID: a.config.ClientID,
		UserPrompt: func(ctx context.Context, message azidentity.DeviceCodeMessage) error {
			a.logger.Info("\n" + string("=")[:50])
			a.logger.Info("OneDrive Authentication Required")
			a.logger.Info(string("=")[:50])
			a.logger.Info("To sign in, use a web browser to open the page:")
			a.logger.Info("  %s", message.VerificationURL)
			a.logger.Info("\nAnd enter the code:")
			a.logger.Info("  %s", message.UserCode)
			a.logger.Info(string("=")[:50] + "\n")
			return nil
		},
	}

	cred, err := azidentity.NewDeviceCodeCredential(options)
	if err != nil {
		return fmt.Errorf("failed to create device code credential: %w", err)
	}

	a.cred = cred

	// Get a token to verify authentication works
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{defaultScope},
	})
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Store the token
	tokenStore := &TokenStore{
		AccessToken: token.Token,
		ExpiresAt:   token.ExpiresOn,
		TokenType:   "Bearer",
	}

	if err := a.saveToken(tokenStore); err != nil {
		a.logger.Debug("Warning: failed to save token: %v", err)
	}

	a.logger.Info("Authentication successful!")
	return nil
}

// GetCredential returns the token credential
func (a *Authenticator) GetCredential() azcore.TokenCredential {
	return a.cred
}

// GetAccessToken gets a valid access token
func (a *Authenticator) GetAccessToken(ctx context.Context) (string, error) {
	if a.cred == nil {
		return "", fmt.Errorf("not authenticated, call Authenticate first")
	}

	token, err := a.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{defaultScope},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return token.Token, nil
}

// saveToken saves the token to disk
func (a *Authenticator) saveToken(token *TokenStore) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(a.tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(a.tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// loadToken loads the token from disk
func (a *Authenticator) loadToken() (*TokenStore, error) {
	data, err := os.ReadFile(a.tokenPath)
	if err != nil {
		return nil, err
	}

	var token TokenStore
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// isTokenExpired checks if token is expired
func (a *Authenticator) isTokenExpired(token *TokenStore) bool {
	// Add 5 minute buffer
	return time.Now().Add(5 * time.Minute).After(token.ExpiresAt)
}

// ClearToken removes the stored token
func (a *Authenticator) ClearToken() error {
	if err := os.Remove(a.tokenPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	a.cred = nil
	return nil
}
