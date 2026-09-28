<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '$lib/stores/auth';
  import { api, type OAuthProvider } from '$lib/api/client';
  import { Lock, Mail, AlertCircle, ArrowRight } from '@lucide/svelte';

  let email = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);
  let oauthProviders = $state<OAuthProvider[]>([]);
  let oauthLoading = $state(false);

  onMount(async () => {
    // 1. Check if token or error is returned in URL hash or query params
    const hash = window.location.hash.substring(1);
    const searchParams = new URLSearchParams(window.location.search);
    const hashParams = new URLSearchParams(hash);

    const token = hashParams.get('token') || searchParams.get('token');
    const redirectUrl = hashParams.get('redirect') || searchParams.get('redirect');
    const errParam = searchParams.get('error') || hashParams.get('error');

    if (errParam) {
      error = decodeURIComponent(errParam);
    }

    if (token) {
      loading = true;
      try {
        await auth.loginWithToken(token, redirectUrl || '/');
        return;
      } catch (err: any) {
        error = err.message || 'Failed to authenticate session from OAuth provider';
        loading = false;
      }
    }

    // 2. Fetch enabled OAuth providers configured by admin
    try {
      oauthProviders = await api.getOAuthProviders();
    } catch {
      oauthProviders = [];
    }
  });

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!email || !password) {
      error = 'Please provide both email and password';
      return;
    }

    loading = true;
    error = '';

    try {
      await auth.login({ email, password });
    } catch (err: any) {
      error = err.message || 'Login failed. Please check your credentials.';
    } finally {
      loading = false;
    }
  }

  function handleOAuthLogin(provider: string) {
    oauthLoading = true;
    window.location.href = `/api/auth/oauth/${provider}/login`;
  }
</script>

<div class="auth-card">
  <div class="auth-header">
    <div class="sidebar-logo-mark auth-logo">K</div>
    <h2>Sign in to Klouds</h2>
    <p class="auth-subtitle">Access your multi-service cloud platform</p>
  </div>

  {#if error}
    <div class="error-banner">
      <AlertCircle size={16} />
      <span>{error}</span>
    </div>
  {/if}

  {#if oauthProviders.length > 0}
    <div class="oauth-section">
      {#each oauthProviders as p}
        <button
          type="button"
          class="btn btn-oauth btn-oauth-{p.provider}"
          onclick={() => handleOAuthLogin(p.provider)}
          disabled={loading || oauthLoading}
        >
          {#if p.provider === 'github'}
            <svg class="oauth-icon" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
            </svg>
          {:else if p.provider === 'gitlab'}
            <svg class="oauth-icon" viewBox="0 0 24 24" fill="#fc6d26">
              <path d="M23.955 13.587l-1.342-4.135-2.664-8.189c-.135-.423-.73-.423-.867 0L16.418 9.45H7.582L4.918 1.263c-.136-.423-.731-.423-.867 0L1.387 9.452.045 13.587c-.121.375.014.789.331 1.023L12 23.054l11.624-8.444c.318-.234.453-.648.331-1.023z"/>
            </svg>
          {:else}
            <svg class="oauth-icon" viewBox="0 0 24 24" fill="#0052cc">
              <path d="M.375 0h23.25c.207 0 .375.168.375.375v4.5a.375.375 0 01-.375.375h-7.662l-1.408 8.447a.375.375 0 01-.37.313H9.815a.375.375 0 01-.37-.313L8.037 5.25H.375A.375.375 0 010 4.875v-4.5C0 .168.168 0 .375 0z"/>
            </svg>
          {/if}
          <span>Continue with {p.name}</span>
        </button>
      {/each}

      <div class="oauth-divider">
        <span class="oauth-divider-line"></span>
        <span class="oauth-divider-text">or continue with email</span>
        <span class="oauth-divider-line"></span>
      </div>
    </div>
  {/if}

  <form onsubmit={handleSubmit} class="auth-form">
    <div class="form-group">
      <label class="form-label" for="login-email">Email Address</label>
      <div class="input-with-icon">
        <Mail size={16} class="input-icon" />
        <input
          id="login-email"
          type="email"
          class="form-input has-icon"
          placeholder="user@domain.internal"
          bind:value={email}
          required
          autocomplete="email"
        />
      </div>
    </div>

    <div class="form-group">
      <label class="form-label" for="login-password">Password</label>
      <div class="input-with-icon">
        <Lock size={16} class="input-icon" />
        <input
          id="login-password"
          type="password"
          class="form-input has-icon"
          placeholder="Password"
          bind:value={password}
          required
          autocomplete="current-password"
        />
      </div>
    </div>

    <button type="submit" class="btn btn-primary w-full" disabled={loading || oauthLoading}>
      {#if loading}
        <span>Authenticating...</span>
      {:else}
        <span>Sign In with Password</span>
        <ArrowRight size={16} />
      {/if}
    </button>
  </form>

  <div class="auth-footer">
    <span class="text-muted">Do not have an account?</span>
    <a href="/register" class="auth-link">Create an account</a>
  </div>
</div>

<style>
  .auth-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 400px;
    padding: 2.25rem 2rem;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.45);
  }

  .auth-header {
    text-align: center;
    margin-bottom: var(--sp-5);
  }

  .auth-logo {
    margin: 0 auto var(--sp-3);
    width: 36px;
    height: 36px;
    font-size: 1.125rem;
  }

  .auth-subtitle {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
    margin-top: 4px;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    background: var(--color-danger-subtle);
    border: 1px solid rgba(248, 113, 113, 0.3);
    border-radius: var(--radius-md);
    color: var(--color-danger);
    font-size: 0.8125rem;
    margin-bottom: var(--sp-4);
  }

  .auth-form {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }

  .input-with-icon {
    position: relative;
    display: flex;
    align-items: center;
  }

  :global(.input-icon) {
    position: absolute;
    left: 12px;
    color: var(--color-ink-muted);
    pointer-events: none;
  }

  .form-input.has-icon {
    padding-left: 38px;
  }

  .auth-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    margin-top: var(--sp-5);
    padding-top: var(--sp-4);
    border-top: 1px solid var(--color-border-subtle);
    font-size: 0.8125rem;
  }

  .auth-link {
    color: var(--color-ink);
    font-weight: 600;
  }

  .auth-link:hover {
    text-decoration: underline;
  }

  .oauth-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: var(--sp-4);
  }

  .btn-oauth {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    width: 100%;
    padding: 10px 16px;
    border-radius: var(--radius-md);
    font-size: 0.875rem;
    font-weight: 500;
    transition: all var(--transition-fast);
    cursor: pointer;
    background: var(--color-surface-hover);
    color: var(--color-ink);
    border: 1px solid var(--color-border);
  }

  .btn-oauth:hover {
    background: var(--color-border);
    border-color: var(--color-border-focus);
    transform: translateY(-1px);
  }

  .oauth-icon {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
  }

  .oauth-divider {
    display: flex;
    align-items: center;
    margin: 8px 0;
    gap: 12px;
  }

  .oauth-divider-line {
    flex: 1;
    height: 1px;
    background: var(--color-border-subtle);
  }

  .oauth-divider-text {
    font-size: 0.75rem;
    color: var(--color-ink-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
</style>
