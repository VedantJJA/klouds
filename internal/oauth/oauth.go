package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrCSRFExpired   = errors.New("OAuth CSRF state token has expired (valid for 10 minutes)")
	ErrCSRFInvalid   = errors.New("invalid OAuth CSRF state signature or tampering detected")
	ErrUnknownProvider = errors.New("unsupported git OAuth provider")
)

// StatePayload represents the data encoded into the OAuth state parameter.
type StatePayload struct {
	Nonce     string `json:"n"`
	Action    string `json:"a"` // "login" or "link"
	UserID    string `json:"u,omitempty"`
	Provider  string `json:"p"`
	ReturnTo  string `json:"r,omitempty"`
	ExpiresAt int64  `json:"exp"` // Unix timestamp
}

// CSRFManager handles creation and validation of 10-minute cryptographically signed OAuth states.
type CSRFManager struct {
	secretKey []byte
	ttl       time.Duration
}

// NewCSRFManager creates a CSRF manager with 10-minute default expiration.
func NewCSRFManager(secretKey string) *CSRFManager {
	h := sha256.Sum256([]byte(secretKey))
	return &CSRFManager{
		secretKey: h[:],
		ttl:       10 * time.Minute, // 10 minute period as required
	}
}

// GenerateState creates a signed state string valid for exactly 10 minutes.
func (cm *CSRFManager) GenerateState(action, userID, provider, returnTo string) (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	payload := StatePayload{
		Nonce:     hex.EncodeToString(nonceBytes),
		Action:    action,
		UserID:    userID,
		Provider:  provider,
		ReturnTo:  returnTo,
		ExpiresAt: time.Now().Add(cm.ttl).Unix(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal state: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(data)

	// HMAC-SHA256 signature
	mac := hmac.New(sha256.New, cm.secretKey)
	mac.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", payloadB64, sigB64), nil
}

// ValidateState validates the signature and ensures the state has not expired.
func (cm *CSRFManager) ValidateState(stateStr, expectedProvider string) (*StatePayload, error) {
	parts := strings.Split(stateStr, ".")
	if len(parts) != 2 {
		return nil, ErrCSRFInvalid
	}

	payloadB64, sigB64 := parts[0], parts[1]

	// Verify HMAC signature
	mac := hmac.New(sha256.New, cm.secretKey)
	mac.Write([]byte(payloadB64))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil || !hmac.Equal(actualSig, expectedSig) {
		return nil, ErrCSRFInvalid
	}

	// Decode payload
	data, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrCSRFInvalid
	}

	var payload StatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, ErrCSRFInvalid
	}

	// Verify expiration (10 min period check)
	if time.Now().Unix() > payload.ExpiresAt {
		return nil, ErrCSRFExpired
	}

	// Verify provider matches
	if expectedProvider != "" && payload.Provider != expectedProvider {
		return nil, ErrCSRFInvalid
	}

	return &payload, nil
}

// UserProfile represents the normalized user profile from a git provider.
type UserProfile struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GitRepo represents a normalized git repository from GitHub, GitLab, or Bitbucket.
type GitRepo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
	Provider      string `json:"provider"` // "github", "gitlab", "bitbucket"
	AvatarURL     string `json:"avatar_url"`
	UpdatedAt     string `json:"updated_at"`
}

// BuildAuthURL constructs the provider's authorization redirect URL.
func BuildAuthURL(provider, clientID, redirectURI, state string, customAuthURL *string) (string, error) {
	switch provider {
	case "github":
		authURL := "https://github.com/login/oauth/authorize"
		if customAuthURL != nil && *customAuthURL != "" {
			authURL = *customAuthURL
		}
		params := url.Values{
			"client_id":    {clientID},
			"redirect_uri": {redirectURI},
			"scope":        {"read:user,user:email,repo"},
			"state":        {state},
		}
		return fmt.Sprintf("%s?%s", authURL, params.Encode()), nil

	case "gitlab":
		authURL := "https://gitlab.com/oauth/authorize"
		if customAuthURL != nil && *customAuthURL != "" {
			authURL = *customAuthURL
		}
		params := url.Values{
			"client_id":     {clientID},
			"redirect_uri":  {redirectURI},
			"response_type": {"code"},
			"scope":         {"read_user read_api read_repository"},
			"state":         {state},
		}
		return fmt.Sprintf("%s?%s", authURL, params.Encode()), nil

	case "bitbucket":
		authURL := "https://bitbucket.org/site/oauth2/authorize"
		if customAuthURL != nil && *customAuthURL != "" {
			authURL = *customAuthURL
		}
		params := url.Values{
			"client_id":     {clientID},
			"response_type": {"code"},
			"state":         {state},
		}
		return fmt.Sprintf("%s?%s", authURL, params.Encode()), nil

	default:
		return "", ErrUnknownProvider
	}
}

// ExchangeToken exchanges an authorization code for an OAuth access token.
func ExchangeToken(ctx context.Context, provider, code, redirectURI, clientID, clientSecret string, customTokenURL *string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	switch provider {
	case "github":
		tokenURL := "https://github.com/login/oauth/access_token"
		if customTokenURL != nil && *customTokenURL != "" {
			tokenURL = *customTokenURL
		}

		form := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"redirect_uri":  {redirectURI},
		}

		req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("github token request: %w", err)
		}
		defer resp.Body.Close()

		var res struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return "", fmt.Errorf("decode github token response: %w", err)
		}
		if res.Error != "" {
			return "", fmt.Errorf("github oauth error: %s (%s)", res.Error, res.ErrorDesc)
		}
		if res.AccessToken == "" {
			return "", errors.New("no access token received from github")
		}
		return res.AccessToken, nil

	case "gitlab":
		tokenURL := "https://gitlab.com/oauth/token"
		if customTokenURL != nil && *customTokenURL != "" {
			tokenURL = *customTokenURL
		}

		form := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURI},
		}

		req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("gitlab token request: %w", err)
		}
		defer resp.Body.Close()

		var res struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return "", fmt.Errorf("decode gitlab token response: %w", err)
		}
		if res.Error != "" {
			return "", fmt.Errorf("gitlab oauth error: %s (%s)", res.Error, res.ErrorDesc)
		}
		return res.AccessToken, nil

	case "bitbucket":
		tokenURL := "https://bitbucket.org/site/oauth2/access_token"
		if customTokenURL != nil && *customTokenURL != "" {
			tokenURL = *customTokenURL
		}

		form := url.Values{
			"grant_type": {"authorization_code"},
			"code":       {code},
		}

		req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.SetBasicAuth(clientID, clientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("bitbucket token request: %w", err)
		}
		defer resp.Body.Close()

		var res struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return "", fmt.Errorf("decode bitbucket token response: %w", err)
		}
		if res.Error != "" {
			return "", fmt.Errorf("bitbucket oauth error: %s", res.Error)
		}
		return res.AccessToken, nil

	default:
		return "", ErrUnknownProvider
	}
}

// FetchUserProfile retrieves the user profile and primary email from the git provider.
func FetchUserProfile(ctx context.Context, provider, token string, customAPIURL *string) (*UserProfile, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	switch provider {
	case "github":
		apiBase := "https://api.github.com"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github user request: %w", err)
		}
		defer resp.Body.Close()

		var ghUser struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
			return nil, fmt.Errorf("decode github user: %w", err)
		}

		email := ghUser.Email
		// If email is private in GitHub profile, fetch from emails endpoint
		if email == "" {
			email = fetchGitHubPrimaryEmail(ctx, client, apiBase, token)
		}

		return &UserProfile{
			ID:        fmt.Sprintf("%d", ghUser.ID),
			Username:  ghUser.Login,
			Name:      ghUser.Name,
			Email:     email,
			AvatarURL: ghUser.AvatarURL,
		}, nil

	case "gitlab":
		apiBase := "https://gitlab.com/api/v4"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gitlab user request: %w", err)
		}
		defer resp.Body.Close()

		var glUser struct {
			ID        int64  `json:"id"`
			Username  string `json:"username"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&glUser); err != nil {
			return nil, fmt.Errorf("decode gitlab user: %w", err)
		}

		return &UserProfile{
			ID:        fmt.Sprintf("%d", glUser.ID),
			Username:  glUser.Username,
			Name:      glUser.Name,
			Email:     glUser.Email,
			AvatarURL: glUser.AvatarURL,
		}, nil

	case "bitbucket":
		apiBase := "https://api.bitbucket.org/2.0"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("bitbucket user request: %w", err)
		}
		defer resp.Body.Close()

		var bbUser struct {
			UUID        string `json:"uuid"`
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Links       struct {
				Avatar struct {
					Href string `json:"href"`
				} `json:"avatar"`
			} `json:"links"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&bbUser); err != nil {
			return nil, fmt.Errorf("decode bitbucket user: %w", err)
		}

		email := fetchBitbucketPrimaryEmail(ctx, client, apiBase, token)

		return &UserProfile{
			ID:        bbUser.UUID,
			Username:  bbUser.Username,
			Name:      bbUser.DisplayName,
			Email:     email,
			AvatarURL: bbUser.Links.Avatar.Href,
		}, nil

	default:
		return nil, ErrUnknownProvider
	}
}

// FetchRepositories lists repositories for the authenticated user from the provider.
func FetchRepositories(ctx context.Context, provider, token string, customAPIURL *string) ([]GitRepo, error) {
	client := &http.Client{Timeout: 20 * time.Second}

	switch provider {
	case "github":
		apiBase := "https://api.github.com"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user/repos?per_page=100&sort=updated&affiliation=owner,collaborator,organization_member", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch github repos: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("github api error (%d): %s", resp.StatusCode, string(body))
		}

		var ghRepos []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			HTMLURL       string `json:"html_url"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
			Description   string `json:"description"`
			Owner         struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
			UpdatedAt string `json:"updated_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
			return nil, fmt.Errorf("decode github repos: %w", err)
		}

		result := make([]GitRepo, 0, len(ghRepos))
		for _, r := range ghRepos {
			result = append(result, GitRepo{
				ID:            fmt.Sprintf("%d", r.ID),
				Name:          r.Name,
				FullName:      r.FullName,
				HTMLURL:       r.HTMLURL,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
				Description:   r.Description,
				Provider:      "github",
				AvatarURL:     r.Owner.AvatarURL,
				UpdatedAt:     r.UpdatedAt,
			})
		}
		return result, nil

	case "gitlab":
		apiBase := "https://gitlab.com/api/v4"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/projects?membership=true&per_page=100&order_by=updated_at", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch gitlab repos: %w", err)
		}
		defer resp.Body.Close()

		var glProjects []struct {
			ID                int64  `json:"id"`
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
			HTTPURLToRepo     string `json:"http_url_to_repo"`
			DefaultBranch     string `json:"default_branch"`
			Visibility        string `json:"visibility"`
			Description       string `json:"description"`
			AvatarURL         string `json:"avatar_url"`
			LastActivityAt    string `json:"last_activity_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&glProjects); err != nil {
			return nil, fmt.Errorf("decode gitlab repos: %w", err)
		}

		result := make([]GitRepo, 0, len(glProjects))
		for _, p := range glProjects {
			branch := p.DefaultBranch
			if branch == "" {
				branch = "main"
			}
			result = append(result, GitRepo{
				ID:            fmt.Sprintf("%d", p.ID),
				Name:          p.Name,
				FullName:      p.PathWithNamespace,
				HTMLURL:       p.WebURL,
				CloneURL:      p.HTTPURLToRepo,
				DefaultBranch: branch,
				Private:       p.Visibility != "public",
				Description:   p.Description,
				Provider:      "gitlab",
				AvatarURL:     p.AvatarURL,
				UpdatedAt:     p.LastActivityAt,
			})
		}
		return result, nil

	case "bitbucket":
		apiBase := "https://api.bitbucket.org/2.0"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/repositories?role=member&pagelen=100&sort=-updated_on", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch bitbucket repos: %w", err)
		}
		defer resp.Body.Close()

		var bbData struct {
			Values []struct {
				UUID       string `json:"uuid"`
				Name       string `json:"name"`
				FullName   string `json:"full_name"`
				IsPrivate  bool   `json:"is_private"`
				UpdatedOn  string `json:"updated_on"`
				MainBranch struct {
					Name string `json:"name"`
				} `json:"mainbranch"`
				Links struct {
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
					Clone []struct {
						Name string `json:"name"`
						Href string `json:"href"`
					} `json:"clone"`
					Avatar struct {
						Href string `json:"href"`
					} `json:"avatar"`
				} `json:"links"`
			} `json:"values"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&bbData); err != nil {
			return nil, fmt.Errorf("decode bitbucket repos: %w", err)
		}

		result := make([]GitRepo, 0, len(bbData.Values))
		for _, v := range bbData.Values {
			cloneURL := ""
			for _, c := range v.Links.Clone {
				if c.Name == "https" {
					cloneURL = c.Href
					break
				}
			}
			branch := v.MainBranch.Name
			if branch == "" {
				branch = "main"
			}
			result = append(result, GitRepo{
				ID:            v.UUID,
				Name:          v.Name,
				FullName:      v.FullName,
				HTMLURL:       v.Links.HTML.Href,
				CloneURL:      cloneURL,
				DefaultBranch: branch,
				Private:       v.IsPrivate,
				Provider:      "bitbucket",
				AvatarURL:     v.Links.Avatar.Href,
				UpdatedAt:     v.UpdatedOn,
			})
		}
		return result, nil

	default:
		return nil, ErrUnknownProvider
	}
}

// FetchBranches retrieves branch names for a repository.
func FetchBranches(ctx context.Context, provider, token, owner, repo string, customAPIURL *string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	switch provider {
	case "github":
		apiBase := "https://api.github.com"
		if customAPIURL != nil && *customAPIURL != "" {
			apiBase = strings.TrimSuffix(*customAPIURL, "/")
		}

		url := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100", apiBase, owner, repo)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var branches []struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
			return nil, err
		}

		names := make([]string, 0, len(branches))
		for _, b := range branches {
			names = append(names, b.Name)
		}
		return names, nil

	default:
		return []string{"main", "master"}, nil
	}
}

func fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client, apiBase, token string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	if len(emails) > 0 {
		return emails[0].Email
	}
	return ""
}

func fetchBitbucketPrimaryEmail(ctx context.Context, client *http.Client, apiBase, token string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var data struct {
		Values []struct {
			Email     string `json:"email"`
			IsPrimary bool   `json:"is_primary"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}

	for _, e := range data.Values {
		if e.IsPrimary {
			return e.Email
		}
	}
	if len(data.Values) > 0 {
		return data.Values[0].Email
	}
	return ""
}
