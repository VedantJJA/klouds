package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/vedant/klouds/internal/auth"
	"github.com/vedant/klouds/internal/db"
	"github.com/vedant/klouds/internal/oauth"
	"github.com/vedant/klouds/internal/secrets"
)

// OAuthHandler coordinates Git OAuth authorization, account linking, and repository retrieval.
type OAuthHandler struct {
	pool      *pgxpool.Pool
	queries   *db.Queries
	tokenSvc  *auth.TokenService
	encryptor *secrets.Encryptor
	csrf      *oauth.CSRFManager
	baseURL   string
}

// NewOAuthHandler creates a new OAuth handler instance.
func NewOAuthHandler(
	pool *pgxpool.Pool,
	tokenSvc *auth.TokenService,
	encryptor *secrets.Encryptor,
	secretKey string,
	baseURL string,
) *OAuthHandler {
	return &OAuthHandler{
		pool:      pool,
		queries:   db.New(pool),
		tokenSvc:  tokenSvc,
		encryptor: encryptor,
		csrf:      oauth.NewCSRFManager(secretKey),
		baseURL:   strings.TrimSuffix(baseURL, "/"),
	}
}

// ProviderConfigDTO represents an OAuth provider setting safe for admin view (no secret leakage).
type ProviderConfigDTO struct {
	Provider  string  `json:"provider"`
	ClientID  string  `json:"client_id"`
	HasSecret bool    `json:"has_secret"`
	AuthURL   *string `json:"auth_url,omitempty"`
	TokenURL  *string `json:"token_url,omitempty"`
	APIURL    *string `json:"api_url,omitempty"`
	Enabled   bool    `json:"enabled"`
	UpdatedAt string  `json:"updated_at"`
}

// UpdateProviderConfigRequest represents payload for updating an OAuth provider.
type UpdateProviderConfigRequest struct {
	ClientID     string  `json:"client_id"`
	ClientSecret string  `json:"client_secret,omitempty"`
	AuthURL      *string `json:"auth_url,omitempty"`
	TokenURL     *string `json:"token_url,omitempty"`
	APIURL       *string `json:"api_url,omitempty"`
	Enabled      bool    `json:"enabled"`
}

// GetProviderConfigs handles GET /api/admin/oauth
func (h *OAuthHandler) GetProviderConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.queries.ListOAuthProviderConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query oauth providers")
		return
	}

	result := make([]ProviderConfigDTO, 0)
	for _, c := range configs {
		result = append(result, ProviderConfigDTO{
			Provider:  c.Provider,
			ClientID:  c.ClientID,
			HasSecret: c.ClientSecretEncrypted != "",
			AuthURL:   c.AuthURL,
			TokenURL:  c.TokenURL,
			APIURL:    c.APIURL,
			Enabled:   c.Enabled,
			UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateProviderConfig handles PUT /api/admin/oauth/{provider}
func (h *OAuthHandler) UpdateProviderConfig(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if provider != "github" && provider != "gitlab" && provider != "bitbucket" {
		writeError(w, http.StatusBadRequest, "invalid oauth provider; must be github, gitlab, or bitbucket")
		return
	}

	var req UpdateProviderConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var secretEncrypted string
	if req.ClientSecret != "" {
		enc, err := h.encryptor.Encrypt(req.ClientSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt client secret")
			return
		}
		secretEncrypted = enc
	}

	err := h.queries.UpsertOAuthProviderConfig(r.Context(), db.OAuthProviderConfig{
		Provider:              provider,
		ClientID:              req.ClientID,
		ClientSecretEncrypted: secretEncrypted,
		AuthURL:               req.AuthURL,
		TokenURL:              req.TokenURL,
		APIURL:                req.APIURL,
		Enabled:               req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update provider config")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "success",
		"provider": provider,
	})
}

// GetEnabledProviders handles GET /api/auth/oauth/providers
func (h *OAuthHandler) GetEnabledProviders(w http.ResponseWriter, r *http.Request) {
	configs, err := h.queries.ListEnabledOAuthProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query enabled providers")
		return
	}

	type EnabledProvider struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
	}

	list := make([]EnabledProvider, 0)
	for _, c := range configs {
		name := strings.Title(c.Provider)
		if c.Provider == "github" {
			name = "GitHub"
		} else if c.Provider == "gitlab" {
			name = "GitLab"
		} else if c.Provider == "bitbucket" {
			name = "Bitbucket"
		}
		list = append(list, EnabledProvider{
			Provider: c.Provider,
			Name:     name,
		})
	}

	writeJSON(w, http.StatusOK, list)
}

// InitiateLogin handles GET /api/auth/oauth/{provider}/login
func (h *OAuthHandler) InitiateLogin(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	h.startOAuthFlow(w, r, provider, "login", "", r.URL.Query().Get("return_to"))
}

// InitiateConnect handles GET /api/user/oauth/{provider}/connect
func (h *OAuthHandler) InitiateConnect(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	provider := strings.ToLower(chi.URLParam(r, "provider"))
	h.startOAuthFlow(w, r, provider, "link", userID, r.URL.Query().Get("return_to"))
}

func (h *OAuthHandler) startOAuthFlow(w http.ResponseWriter, r *http.Request, provider, action, userID, returnTo string) {
	cfg, err := h.queries.GetOAuthProviderConfig(r.Context(), provider)
	if err != nil || !cfg.Enabled || cfg.ClientID == "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("OAuth provider is not configured or disabled"), http.StatusTemporaryRedirect)
		return
	}

	// Generate CSRF token valid for exactly 10 minutes
	state, err := h.csrf.GenerateState(action, userID, provider, returnTo)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to generate secure OAuth state"), http.StatusTemporaryRedirect)
		return
	}

	// Set 10-minute HTTP-only CSRF verification cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/api/auth/oauth",
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.baseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes TTL
	})

	redirectURI := fmt.Sprintf("%s/api/auth/oauth/%s/callback", h.baseURL, provider)
	authURL, err := oauth.BuildAuthURL(provider, cfg.ClientID, redirectURI, state, cfg.AuthURL)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to build authorization URL"), http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback handles GET /api/auth/oauth/{provider}/callback
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))

	// Check if OAuth provider returned an error
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		errDesc := r.URL.Query().Get("error_description")
		msg := oauthErr
		if errDesc != "" {
			msg += ": " + errDesc
		}
		http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")

	if code == "" || stateParam == "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Missing code or state parameter"), http.StatusTemporaryRedirect)
		return
	}

	// Validate CSRF state token with strict 10-minute expiry check
	statePayload, err := h.csrf.ValidateState(stateParam, provider)
	if err != nil {
		errMsg := "Invalid OAuth CSRF state"
		if err == oauth.ErrCSRFExpired {
			errMsg = "OAuth session expired (limit is 10 minutes). Please try again."
		}
		http.Redirect(w, r, "/login?error="+url.QueryEscape(errMsg), http.StatusTemporaryRedirect)
		return
	}

	// Clear CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/api/auth/oauth",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Fetch provider configuration & decrypt client secret
	cfg, err := h.queries.GetOAuthProviderConfig(r.Context(), provider)
	if err != nil || cfg.ClientSecretEncrypted == "" {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Provider configuration is incomplete"), http.StatusTemporaryRedirect)
		return
	}

	clientSecret, err := h.encryptor.Decrypt(cfg.ClientSecretEncrypted)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to decrypt provider credentials"), http.StatusTemporaryRedirect)
		return
	}

	redirectURI := fmt.Sprintf("%s/api/auth/oauth/%s/callback", h.baseURL, provider)
	accessToken, err := oauth.ExchangeToken(r.Context(), provider, code, redirectURI, cfg.ClientID, clientSecret, cfg.TokenURL)
	if err != nil {
		log.Error().Err(err).Str("provider", provider).Msg("Failed to exchange OAuth token")
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to authenticate with git provider: "+err.Error()), http.StatusTemporaryRedirect)
		return
	}

	// Fetch user profile from git provider
	profile, err := oauth.FetchUserProfile(r.Context(), provider, accessToken, cfg.APIURL)
	if err != nil {
		log.Error().Err(err).Str("provider", provider).Msg("Failed to fetch user profile")
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to fetch profile from git provider"), http.StatusTemporaryRedirect)
		return
	}

	tokenEncrypted, err := h.encryptor.Encrypt(accessToken)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to encrypt access token"), http.StatusTemporaryRedirect)
		return
	}

	// Handle Action: Link to existing logged-in user
	if statePayload.Action == "link" {
		if statePayload.UserID == "" {
			http.Redirect(w, r, "/settings?error="+url.QueryEscape("Missing user context for linking"), http.StatusTemporaryRedirect)
			return
		}

		err = h.queries.UpsertUserOAuthAccount(r.Context(), db.UserOAuthAccount{
			UserID:               statePayload.UserID,
			Provider:             provider,
			ProviderUserID:       profile.ID,
			ProviderUsername:     profile.Username,
			ProviderEmail:        profile.Email,
			AvatarURL:            profile.AvatarURL,
			AccessTokenEncrypted: tokenEncrypted,
			Scopes:               []string{"read:user", "repo"},
		})
		if err != nil {
			http.Redirect(w, r, "/settings?error="+url.QueryEscape("Failed to link git account"), http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, "/settings?connected="+provider, http.StatusTemporaryRedirect)
		return
	}

	// Handle Action: Login / Register
	var targetUserID string
	var targetUsername string
	var targetRole string

	// 1. Check if OAuth account is already linked
	existingOAuth, err := h.queries.GetUserOAuthAccountByProviderUserID(r.Context(), provider, profile.ID)
	if err == nil {
		targetUserID = existingOAuth.UserID
		// Fetch user
		u, err := h.queries.GetUserByID(r.Context(), targetUserID)
		if err == nil {
			targetUsername = u.Username
			targetRole = u.Role
		}
	}

	// 2. If not linked by provider ID, check if user exists with matching email
	if targetUserID == "" && profile.Email != "" {
		u, err := h.queries.GetUserByEmail(r.Context(), profile.Email)
		if err == nil {
			targetUserID = u.ID
			targetUsername = u.Username
			targetRole = u.Role
		}
	}

	// 3. If user still does not exist, auto-create new account
	if targetUserID == "" {
		userCount, _ := h.queries.CountUsers(r.Context())
		role := "user"
		status := "active"
		if userCount == 0 {
			role = "admin" // First user is automatically platform admin
		}

		cleanUsername := strings.ToLower(profile.Username)
		if cleanUsername == "" {
			cleanUsername = "user" + strings.ToLower(profile.ID[:6])
		}

		// Ensure username uniqueness
		for i := 0; i < 5; i++ {
			candidate := cleanUsername
			if i > 0 {
				candidate = fmt.Sprintf("%s%d", cleanUsername, i)
			}
			dummyPasswordHash, _ := auth.HashPassword("oauth-random-placeholder-" + time.Now().String())
			email := profile.Email
			if email == "" {
				email = fmt.Sprintf("%s@users.noreply.%s", candidate, provider)
			}

			u, err := h.queries.CreateUser(r.Context(), db.CreateUserParams{
				Email:        email,
				Username:     candidate,
				PasswordHash: dummyPasswordHash,
				Role:         role,
				Status:       status,
			})
			if err == nil {
				targetUserID = u.ID
				targetUsername = u.Username
				targetRole = u.Role
				break
			}
		}

		if targetUserID == "" {
			http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to create user account"), http.StatusTemporaryRedirect)
			return
		}

		// Initialize default user quotas
		if role == "admin" {
			_, _ = h.queries.CreateUserQuota(r.Context(), defaultAdminQuota(targetUserID))
		} else {
			_, _ = h.queries.CreateUserQuota(r.Context(), defaultUserQuota(targetUserID))
		}
	}

	// Update or insert linked OAuth account
	_ = h.queries.UpsertUserOAuthAccount(r.Context(), db.UserOAuthAccount{
		UserID:               targetUserID,
		Provider:             provider,
		ProviderUserID:       profile.ID,
		ProviderUsername:     profile.Username,
		ProviderEmail:        profile.Email,
		AvatarURL:            profile.AvatarURL,
		AccessTokenEncrypted: tokenEncrypted,
		Scopes:               []string{"read:user", "repo"},
	})

	// Generate Klouds JWT auth token
	token, err := h.tokenSvc.CreateToken(targetUserID, targetUsername, targetRole)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("Failed to generate session token"), http.StatusTemporaryRedirect)
		return
	}

	// Redirect to frontend with token in hash (safe from server access logs)
	returnURL := "/login#token=" + token
	if statePayload.ReturnTo != "" && !strings.Contains(statePayload.ReturnTo, "://") {
		returnURL += "&redirect=" + url.QueryEscape(statePayload.ReturnTo)
	}
	http.Redirect(w, r, returnURL, http.StatusTemporaryRedirect)
}

// GetUserOAuthAccounts handles GET /api/user/oauth
func (h *OAuthHandler) GetUserOAuthAccounts(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accounts, err := h.queries.ListUserOAuthAccounts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch linked accounts")
		return
	}

	type UserAccountDTO struct {
		Provider  string `json:"provider"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		CreatedAt string `json:"created_at"`
	}

	result := make([]UserAccountDTO, 0)
	for _, a := range accounts {
		result = append(result, UserAccountDTO{
			Provider:  a.Provider,
			Username:  a.ProviderUsername,
			Email:     a.ProviderEmail,
			AvatarURL: a.AvatarURL,
			CreatedAt: a.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// DisconnectOAuthAccount handles DELETE /api/user/oauth/{provider}
func (h *OAuthHandler) DisconnectOAuthAccount(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	provider := strings.ToLower(chi.URLParam(r, "provider"))
	if err := h.queries.DeleteUserOAuthAccount(r.Context(), userID, provider); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect provider")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "disconnected",
		"provider": provider,
	})
}

// ListRepositories handles GET /api/git/repos
// Aggregates repositories across all git hosts the user has authorized in settings.
// The user does not need to choose a provider; repositories automatically appear.
func (h *OAuthHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accounts, err := h.queries.ListUserOAuthAccounts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve git credentials")
		return
	}

	if len(accounts) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"repos":               []oauth.GitRepo{},
			"connected_providers": []string{},
			"message":             "No git accounts connected yet. Authorize GitHub, GitLab, or Bitbucket in Settings.",
		})
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	allRepos := make([]oauth.GitRepo, 0)
	connectedProviders := make([]string, 0)

	for _, acc := range accounts {
		connectedProviders = append(connectedProviders, acc.Provider)
		wg.Add(1)

		go func(account db.UserOAuthAccount) {
			defer wg.Done()

			token, decErr := h.encryptor.Decrypt(account.AccessTokenEncrypted)
			if decErr != nil {
				log.Warn().Str("provider", account.Provider).Err(decErr).Msg("Failed to decrypt oauth token")
				return
			}

			// Query custom API URL if configured
			var customAPIURL *string
			if pCfg, cfgErr := h.queries.GetOAuthProviderConfig(context.Background(), account.Provider); cfgErr == nil {
				customAPIURL = pCfg.APIURL
			}

			repos, repoErr := oauth.FetchRepositories(r.Context(), account.Provider, token, customAPIURL)
			if repoErr != nil {
				log.Warn().Str("provider", account.Provider).Err(repoErr).Msg("Failed to fetch repositories")
				return
			}

			mu.Lock()
			allRepos = append(allRepos, repos...)
			mu.Unlock()
		}(acc)
	}

	wg.Wait()

	// Sort repos by updated_at descending
	// Simple ISO time comparison
	for i := 0; i < len(allRepos)-1; i++ {
		for j := i + 1; j < len(allRepos); j++ {
			if allRepos[i].UpdatedAt < allRepos[j].UpdatedAt {
				allRepos[i], allRepos[j] = allRepos[j], allRepos[i]
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"repos":               allRepos,
		"connected_providers": connectedProviders,
	})
}

// ListBranches handles GET /api/git/branches
func (h *OAuthHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	repoURL := r.URL.Query().Get("repo_url")
	if repoURL == "" {
		writeJSON(w, http.StatusOK, []string{"main", "master"})
		return
	}

	// Parse owner/repo from URL
	provider, owner, repo := parseRepoURL(repoURL)
	if provider == "" || owner == "" || repo == "" {
		writeJSON(w, http.StatusOK, []string{"main", "master"})
		return
	}

	// Check if user has an account connected for this provider
	token := ""
	if acc, err := h.queries.GetUserOAuthAccount(r.Context(), userID, provider); err == nil {
		if decrypted, decErr := h.encryptor.Decrypt(acc.AccessTokenEncrypted); decErr == nil {
			token = decrypted
		}
	}

	var customAPIURL *string
	if pCfg, cfgErr := h.queries.GetOAuthProviderConfig(r.Context(), provider); cfgErr == nil {
		customAPIURL = pCfg.APIURL
	}

	branches, err := oauth.FetchBranches(r.Context(), provider, token, owner, repo, customAPIURL)
	if err != nil || len(branches) == 0 {
		writeJSON(w, http.StatusOK, []string{"main", "master"})
		return
	}

	writeJSON(w, http.StatusOK, branches)
}

func parseRepoURL(raw string) (provider, owner, repo string) {
	clean := strings.TrimSuffix(strings.TrimSpace(raw), ".git")
	clean = strings.TrimPrefix(clean, "git@github.com:")
	clean = strings.TrimPrefix(clean, "git@gitlab.com:")
	clean = strings.TrimPrefix(clean, "git@bitbucket.org:")

	u, err := url.Parse(clean)
	if err == nil && u.Host != "" {
		if strings.Contains(u.Host, "github") {
			provider = "github"
		} else if strings.Contains(u.Host, "gitlab") {
			provider = "gitlab"
		} else if strings.Contains(u.Host, "bitbucket") {
			provider = "bitbucket"
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 {
			owner = parts[0]
			repo = parts[1]
		}
		return
	}

	parts := strings.Split(clean, "/")
	if len(parts) >= 2 {
		provider = "github"
		owner = parts[len(parts)-2]
		repo = parts[len(parts)-1]
	}
	return
}
