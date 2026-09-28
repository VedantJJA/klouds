package db

import (
	"context"
)

// UpsertOAuthProviderConfig creates or updates an OAuth provider configuration (Admin setting).
func (q *Queries) UpsertOAuthProviderConfig(ctx context.Context, cfg OAuthProviderConfig) error {
	query := `
		INSERT INTO oauth_provider_configs (
			provider, client_id, client_secret_encrypted, auth_url, token_url, api_url, enabled, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (provider) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			client_secret_encrypted = CASE WHEN EXCLUDED.client_secret_encrypted <> '' THEN EXCLUDED.client_secret_encrypted ELSE oauth_provider_configs.client_secret_encrypted END,
			auth_url = EXCLUDED.auth_url,
			token_url = EXCLUDED.token_url,
			api_url = EXCLUDED.api_url,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()`

	_, err := q.db.Exec(ctx, query,
		cfg.Provider, cfg.ClientID, cfg.ClientSecretEncrypted,
		cfg.AuthURL, cfg.TokenURL, cfg.APIURL, cfg.Enabled,
	)
	return err
}

// GetOAuthProviderConfig returns an OAuth provider config by name.
func (q *Queries) GetOAuthProviderConfig(ctx context.Context, provider string) (OAuthProviderConfig, error) {
	query := `
		SELECT provider, client_id, client_secret_encrypted, auth_url, token_url, api_url, enabled, created_at, updated_at
		FROM oauth_provider_configs
		WHERE provider = $1`

	var cfg OAuthProviderConfig
	err := q.db.QueryRow(ctx, query, provider).Scan(
		&cfg.Provider, &cfg.ClientID, &cfg.ClientSecretEncrypted,
		&cfg.AuthURL, &cfg.TokenURL, &cfg.APIURL, &cfg.Enabled,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	return cfg, err
}

// ListOAuthProviderConfigs returns all OAuth provider configs.
func (q *Queries) ListOAuthProviderConfigs(ctx context.Context) ([]OAuthProviderConfig, error) {
	query := `
		SELECT provider, client_id, client_secret_encrypted, auth_url, token_url, api_url, enabled, created_at, updated_at
		FROM oauth_provider_configs
		ORDER BY provider ASC`

	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OAuthProviderConfig
	for rows.Next() {
		var cfg OAuthProviderConfig
		if err := rows.Scan(
			&cfg.Provider, &cfg.ClientID, &cfg.ClientSecretEncrypted,
			&cfg.AuthURL, &cfg.TokenURL, &cfg.APIURL, &cfg.Enabled,
			&cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, cfg)
	}
	return list, rows.Err()
}

// ListEnabledOAuthProviders returns all enabled OAuth provider configs.
func (q *Queries) ListEnabledOAuthProviders(ctx context.Context) ([]OAuthProviderConfig, error) {
	query := `
		SELECT provider, client_id, client_secret_encrypted, auth_url, token_url, api_url, enabled, created_at, updated_at
		FROM oauth_provider_configs
		WHERE enabled = true
		ORDER BY provider ASC`

	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OAuthProviderConfig
	for rows.Next() {
		var cfg OAuthProviderConfig
		if err := rows.Scan(
			&cfg.Provider, &cfg.ClientID, &cfg.ClientSecretEncrypted,
			&cfg.AuthURL, &cfg.TokenURL, &cfg.APIURL, &cfg.Enabled,
			&cfg.CreatedAt, &cfg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, cfg)
	}
	return list, rows.Err()
}

// UpsertUserOAuthAccount connects or updates a user's linked OAuth account.
func (q *Queries) UpsertUserOAuthAccount(ctx context.Context, acc UserOAuthAccount) error {
	query := `
		INSERT INTO user_oauth_accounts (
			user_id, provider, provider_user_id, provider_username, provider_email,
			avatar_url, access_token_encrypted, refresh_token_encrypted, token_expires_at, scopes, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (user_id, provider) DO UPDATE SET
			provider_user_id = EXCLUDED.provider_user_id,
			provider_username = EXCLUDED.provider_username,
			provider_email = EXCLUDED.provider_email,
			avatar_url = EXCLUDED.avatar_url,
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
			token_expires_at = EXCLUDED.token_expires_at,
			scopes = EXCLUDED.scopes,
			updated_at = NOW()`

	_, err := q.db.Exec(ctx, query,
		acc.UserID, acc.Provider, acc.ProviderUserID, acc.ProviderUsername, acc.ProviderEmail,
		acc.AvatarURL, acc.AccessTokenEncrypted, acc.RefreshTokenEncrypted, acc.TokenExpiresAt, acc.Scopes,
	)
	return err
}

// GetUserOAuthAccount retrieves a linked account by user ID and provider.
func (q *Queries) GetUserOAuthAccount(ctx context.Context, userID, provider string) (UserOAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, provider_username, provider_email,
		       avatar_url, access_token_encrypted, refresh_token_encrypted, token_expires_at, scopes, created_at, updated_at
		FROM user_oauth_accounts
		WHERE user_id = $1 AND provider = $2`

	var acc UserOAuthAccount
	err := q.db.QueryRow(ctx, query, userID, provider).Scan(
		&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID, &acc.ProviderUsername, &acc.ProviderEmail,
		&acc.AvatarURL, &acc.AccessTokenEncrypted, &acc.RefreshTokenEncrypted, &acc.TokenExpiresAt, &acc.Scopes,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	return acc, err
}

// GetUserOAuthAccountByProviderUserID retrieves an account by provider and provider's user ID.
func (q *Queries) GetUserOAuthAccountByProviderUserID(ctx context.Context, provider, providerUserID string) (UserOAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, provider_username, provider_email,
		       avatar_url, access_token_encrypted, refresh_token_encrypted, token_expires_at, scopes, created_at, updated_at
		FROM user_oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2`

	var acc UserOAuthAccount
	err := q.db.QueryRow(ctx, query, provider, providerUserID).Scan(
		&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID, &acc.ProviderUsername, &acc.ProviderEmail,
		&acc.AvatarURL, &acc.AccessTokenEncrypted, &acc.RefreshTokenEncrypted, &acc.TokenExpiresAt, &acc.Scopes,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	return acc, err
}

// ListUserOAuthAccounts returns all linked OAuth accounts for a user.
func (q *Queries) ListUserOAuthAccounts(ctx context.Context, userID string) ([]UserOAuthAccount, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, provider_username, provider_email,
		       avatar_url, access_token_encrypted, refresh_token_encrypted, token_expires_at, scopes, created_at, updated_at
		FROM user_oauth_accounts
		WHERE user_id = $1
		ORDER BY provider ASC`

	rows, err := q.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []UserOAuthAccount
	for rows.Next() {
		var acc UserOAuthAccount
		if err := rows.Scan(
			&acc.ID, &acc.UserID, &acc.Provider, &acc.ProviderUserID, &acc.ProviderUsername, &acc.ProviderEmail,
			&acc.AvatarURL, &acc.AccessTokenEncrypted, &acc.RefreshTokenEncrypted, &acc.TokenExpiresAt, &acc.Scopes,
			&acc.CreatedAt, &acc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, acc)
	}
	return list, rows.Err()
}

// DeleteUserOAuthAccount unlinks a provider account from a user.
func (q *Queries) DeleteUserOAuthAccount(ctx context.Context, userID, provider string) error {
	query := `DELETE FROM user_oauth_accounts WHERE user_id = $1 AND provider = $2`
	_, err := q.db.Exec(ctx, query, userID, provider)
	return err
}
