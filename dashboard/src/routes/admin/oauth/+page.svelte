<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type AdminOAuthConfig } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Key,
    Check,
    AlertCircle,
    Copy,
    ExternalLink,
    Lock,
    Save
  } from '@lucide/svelte';

  let configs = $state<Record<string, AdminOAuthConfig>>({});
  let loading = $state(true);
  let savingProvider = $state<string | null>(null);
  let successMsg = $state('');
  let errorMsg = $state('');

  // Form states
  let formState = $state({
    github: {
      client_id: '',
      client_secret: '',
      enabled: false
    },
    gitlab: {
      client_id: '',
      client_secret: '',
      auth_url: 'https://gitlab.com/oauth/authorize',
      token_url: 'https://gitlab.com/oauth/token',
      api_url: 'https://gitlab.com/api/v4',
      enabled: false
    },
    bitbucket: {
      client_id: '',
      client_secret: '',
      enabled: false
    }
  });

  let copiedProvider = $state<string | null>(null);

  onMount(async () => {
    await loadConfigs();
  });

  async function loadConfigs() {
    loading = true;
    errorMsg = '';
    try {
      const list = await api.getAdminOAuthConfigs();
      const map: Record<string, AdminOAuthConfig> = {};
      for (const c of list) {
        map[c.provider] = c;
      }
      configs = map;

      // Populate form state
      if (configs.github) {
        formState.github.client_id = configs.github.client_id || '';
        formState.github.enabled = configs.github.enabled;
      }
      if (configs.gitlab) {
        formState.gitlab.client_id = configs.gitlab.client_id || '';
        formState.gitlab.auth_url = configs.gitlab.auth_url || 'https://gitlab.com/oauth/authorize';
        formState.gitlab.token_url = configs.gitlab.token_url || 'https://gitlab.com/oauth/token';
        formState.gitlab.api_url = configs.gitlab.api_url || 'https://gitlab.com/api/v4';
        formState.gitlab.enabled = configs.gitlab.enabled;
      }
      if (configs.bitbucket) {
        formState.bitbucket.client_id = configs.bitbucket.client_id || '';
        formState.bitbucket.enabled = configs.bitbucket.enabled;
      }
    } catch (err: any) {
      errorMsg = err.message || 'Failed to load OAuth configurations';
    } finally {
      loading = false;
    }
  }

  function getCallbackURL(provider: string) {
    if (typeof window !== 'undefined') {
      return `${window.location.origin}/api/auth/oauth/${provider}/callback`;
    }
    return `https://klouds.online/api/auth/oauth/${provider}/callback`;
  }

  async function copyCallback(provider: string) {
    const url = getCallbackURL(provider);
    await navigator.clipboard.writeText(url);
    copiedProvider = provider;
    setTimeout(() => {
      if (copiedProvider === provider) copiedProvider = null;
    }, 2000);
  }

  async function saveConfig(provider: 'github' | 'gitlab' | 'bitbucket') {
    savingProvider = provider;
    successMsg = '';
    errorMsg = '';

    const payload: any = {
      client_id: formState[provider].client_id,
      enabled: formState[provider].enabled
    };

    if (formState[provider].client_secret) {
      payload.client_secret = formState[provider].client_secret;
    }

    if (provider === 'gitlab') {
      payload.auth_url = formState.gitlab.auth_url;
      payload.token_url = formState.gitlab.token_url;
      payload.api_url = formState.gitlab.api_url;
    }

    try {
      await api.updateAdminOAuthConfig(provider, payload);
      successMsg = `Successfully updated ${provider.toUpperCase()} OAuth settings.`;
      // Clear password input once saved
      formState[provider].client_secret = '';
      await loadConfigs();
    } catch (err: any) {
      errorMsg = err.message || `Failed to update ${provider} settings`;
    } finally {
      savingProvider = null;
    }
  }
</script>

<div class="page-container">
  <Breadcrumbs
    items={[
      { label: 'Admin', href: '/admin/users' },
      { label: 'Git OAuth Configuration', href: '/admin/oauth' }
    ]}
  />

  <div class="page-header">
    <div>
      <h1 class="page-title">Git OAuth Configuration</h1>
      <p class="page-subtitle">
        Set up OAuth Apps for GitHub, GitLab, and Bitbucket. Once configured, users can log in via Single Sign-On and browse all their hosted repositories directly during service creation.
      </p>
    </div>
  </div>

  {#if successMsg}
    <div class="alert alert-success">
      <Check size={18} />
      <span>{successMsg}</span>
    </div>
  {/if}

  {#if errorMsg}
    <div class="alert alert-danger">
      <AlertCircle size={18} />
      <span>{errorMsg}</span>
    </div>
  {/if}

  {#if loading}
    <div class="loading-state">
      <div class="spinner"></div>
      <p>Loading OAuth provider configuration...</p>
    </div>
  {:else}
    <div class="oauth-cards-grid">
      <!-- GITHUB OAUTH -->
      <div class="provider-card">
        <div class="provider-header">
          <div class="provider-icon-wrapper github">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
            </svg>
          </div>
          <div class="provider-title-area">
            <h3>GitHub OAuth App</h3>
            <span class="provider-badge {formState.github.enabled ? 'active' : 'inactive'}">
              {formState.github.enabled ? 'Active / Enabled' : 'Disabled'}
            </span>
          </div>
          <label class="toggle-switch">
            <input type="checkbox" bind:checked={formState.github.enabled} />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="provider-body">
          <div class="instruction-box">
            <p><strong>Setup Instructions:</strong></p>
            <ol>
              <li>Go to <strong>GitHub &gt; Settings &gt; Developer settings &gt; OAuth Apps &gt; New OAuth App</strong>.</li>
              <li>Set <strong>Homepage URL</strong> to <code>https://klouds.online</code>.</li>
              <li>Set <strong>Authorization callback URL</strong> to the callback below.</li>
            </ol>
          </div>

          <div class="form-group">
            <label class="form-label" for="gh-callback">Authorization Callback URL</label>
            <div class="copy-input-row">
              <input id="gh-callback" type="text" class="form-input readonly-input" readonly value={getCallbackURL('github')} />
              <button type="button" class="btn btn-secondary" onclick={() => copyCallback('github')}>
                {#if copiedProvider === 'github'}
                  <Check size={16} />
                  <span>Copied</span>
                {:else}
                  <Copy size={16} />
                  <span>Copy</span>
                {/if}
              </button>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="gh-client-id">Client ID</label>
            <input
              id="gh-client-id"
              type="text"
              class="form-input"
              placeholder="e.g. Iv1.8a2b3c4d5e6f7g8h"
              bind:value={formState.github.client_id}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="gh-client-secret">
              Client Secret
              {#if configs.github?.has_secret}
                <span class="secret-indicator"><Lock size={12} /> Encrypted Secret Configured</span>
              {/if}
            </label>
            <input
              id="gh-client-secret"
              type="password"
              class="form-input"
              placeholder={configs.github?.has_secret ? '•••••••••••••••••••••••••••••••• (Leave blank to keep existing)' : 'Enter Client Secret'}
              bind:value={formState.github.client_secret}
            />
          </div>
        </div>

        <div class="provider-footer">
          <a
            href="https://github.com/settings/developers"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-ghost"
          >
            <span>GitHub Developer Settings</span>
            <ExternalLink size={14} />
          </a>
          <button
            type="button"
            class="btn btn-primary"
            disabled={savingProvider === 'github'}
            onclick={() => saveConfig('github')}
          >
            <Save size={16} />
            <span>{savingProvider === 'github' ? 'Saving...' : 'Save GitHub Config'}</span>
          </button>
        </div>
      </div>

      <!-- GITLAB OAUTH -->
      <div class="provider-card">
        <div class="provider-header">
          <div class="provider-icon-wrapper gitlab">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="#fc6d26">
              <path d="M23.955 13.587l-1.342-4.135-2.664-8.189c-.135-.423-.73-.423-.867 0L16.418 9.45H7.582L4.918 1.263c-.136-.423-.731-.423-.867 0L1.387 9.452.045 13.587c-.121.375.014.789.331 1.023L12 23.054l11.624-8.444c.318-.234.453-.648.331-1.023z"/>
            </svg>
          </div>
          <div class="provider-title-area">
            <h3>GitLab OAuth App</h3>
            <span class="provider-badge {formState.gitlab.enabled ? 'active' : 'inactive'}">
              {formState.gitlab.enabled ? 'Active / Enabled' : 'Disabled'}
            </span>
          </div>
          <label class="toggle-switch">
            <input type="checkbox" bind:checked={formState.gitlab.enabled} />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="provider-body">
          <div class="instruction-box">
            <p><strong>Setup Instructions:</strong></p>
            <ol>
              <li>Go to <strong>GitLab &gt; User Settings &gt; Applications</strong>.</li>
              <li>Set <strong>Redirect URI</strong> to the callback URL below.</li>
              <li>Check scopes: <code>read_user</code>, <code>read_api</code>, <code>read_repository</code>.</li>
            </ol>
          </div>

          <div class="form-group">
            <label class="form-label" for="gl-callback">Authorization Callback URL</label>
            <div class="copy-input-row">
              <input id="gl-callback" type="text" class="form-input readonly-input" readonly value={getCallbackURL('gitlab')} />
              <button type="button" class="btn btn-secondary" onclick={() => copyCallback('gitlab')}>
                {#if copiedProvider === 'gitlab'}
                  <Check size={16} />
                  <span>Copied</span>
                {:else}
                  <Copy size={16} />
                  <span>Copy</span>
                {/if}
              </button>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="gl-client-id">Application ID</label>
            <input
              id="gl-client-id"
              type="text"
              class="form-input"
              placeholder="GitLab Application ID"
              bind:value={formState.gitlab.client_id}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="gl-client-secret">
              Secret
              {#if configs.gitlab?.has_secret}
                <span class="secret-indicator"><Lock size={12} /> Encrypted Secret Configured</span>
              {/if}
            </label>
            <input
              id="gl-client-secret"
              type="password"
              class="form-input"
              placeholder={configs.gitlab?.has_secret ? '•••••••••••••••••••••••••••••••• (Leave blank to keep existing)' : 'Enter Secret'}
              bind:value={formState.gitlab.client_secret}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="gl-api-url">GitLab API URL (Optional, for Self-Hosted)</label>
            <input
              id="gl-api-url"
              type="text"
              class="form-input"
              placeholder="https://gitlab.com/api/v4"
              bind:value={formState.gitlab.api_url}
            />
          </div>
        </div>

        <div class="provider-footer">
          <a
            href="https://gitlab.com/-/profile/applications"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-ghost"
          >
            <span>GitLab Applications</span>
            <ExternalLink size={14} />
          </a>
          <button
            type="button"
            class="btn btn-primary"
            disabled={savingProvider === 'gitlab'}
            onclick={() => saveConfig('gitlab')}
          >
            <Save size={16} />
            <span>{savingProvider === 'gitlab' ? 'Saving...' : 'Save GitLab Config'}</span>
          </button>
        </div>
      </div>

      <!-- BITBUCKET OAUTH -->
      <div class="provider-card">
        <div class="provider-header">
          <div class="provider-icon-wrapper bitbucket">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="#0052cc">
              <path d="M.375 0h23.25c.207 0 .375.168.375.375v4.5a.375.375 0 01-.375.375h-7.662l-1.408 8.447a.375.375 0 01-.37.313H9.815a.375.375 0 01-.37-.313L8.037 5.25H.375A.375.375 0 010 4.875v-4.5C0 .168.168 0 .375 0z"/>
            </svg>
          </div>
          <div class="provider-title-area">
            <h3>Bitbucket OAuth App</h3>
            <span class="provider-badge {formState.bitbucket.enabled ? 'active' : 'inactive'}">
              {formState.bitbucket.enabled ? 'Active / Enabled' : 'Disabled'}
            </span>
          </div>
          <label class="toggle-switch">
            <input type="checkbox" bind:checked={formState.bitbucket.enabled} />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="provider-body">
          <div class="instruction-box">
            <p><strong>Setup Instructions:</strong></p>
            <ol>
              <li>Go to <strong>Bitbucket &gt; Personal Settings &gt; OAuth consumers &gt; Add consumer</strong>.</li>
              <li>Set <strong>Callback URL</strong> to the callback below.</li>
              <li>Permissions: <strong>Account (Email, Read)</strong>, <strong>Repositories (Read)</strong>.</li>
            </ol>
          </div>

          <div class="form-group">
            <label class="form-label" for="bb-callback">Authorization Callback URL</label>
            <div class="copy-input-row">
              <input id="bb-callback" type="text" class="form-input readonly-input" readonly value={getCallbackURL('bitbucket')} />
              <button type="button" class="btn btn-secondary" onclick={() => copyCallback('bitbucket')}>
                {#if copiedProvider === 'bitbucket'}
                  <Check size={16} />
                  <span>Copied</span>
                {:else}
                  <Copy size={16} />
                  <span>Copy</span>
                {/if}
              </button>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="bb-client-id">Key (Client ID)</label>
            <input
              id="bb-client-id"
              type="text"
              class="form-input"
              placeholder="Bitbucket OAuth Key"
              bind:value={formState.bitbucket.client_id}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="bb-client-secret">
              Secret
              {#if configs.bitbucket?.has_secret}
                <span class="secret-indicator"><Lock size={12} /> Encrypted Secret Configured</span>
              {/if}
            </label>
            <input
              id="bb-client-secret"
              type="password"
              class="form-input"
              placeholder={configs.bitbucket?.has_secret ? '•••••••••••••••••••••••••••••••• (Leave blank to keep existing)' : 'Enter Secret'}
              bind:value={formState.bitbucket.client_secret}
            />
          </div>
        </div>

        <div class="provider-footer">
          <a
            href="https://bitbucket.org/account/settings/api"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-ghost"
          >
            <span>Bitbucket Settings</span>
            <ExternalLink size={14} />
          </a>
          <button
            type="button"
            class="btn btn-primary"
            disabled={savingProvider === 'bitbucket'}
            onclick={() => saveConfig('bitbucket')}
          >
            <Save size={16} />
            <span>{savingProvider === 'bitbucket' ? 'Saving...' : 'Save Bitbucket Config'}</span>
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: var(--sp-6) var(--sp-6);
  }

  .page-header {
    margin-bottom: var(--sp-6);
  }

  .page-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-ink);
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-ink-secondary);
    margin-top: 4px;
    max-width: 800px;
    line-height: 1.5;
  }

  .alert {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: var(--radius-md);
    margin-bottom: var(--sp-5);
    font-size: 0.875rem;
  }

  .alert-success {
    background: rgba(34, 197, 94, 0.12);
    border: 1px solid rgba(34, 197, 94, 0.3);
    color: #4ade80;
  }

  .alert-danger {
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171;
  }

  .oauth-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
    gap: var(--sp-6);
  }

  .provider-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
  }

  .provider-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--color-border-subtle);
    background: rgba(255, 255, 255, 0.02);
  }

  .provider-icon-wrapper {
    width: 38px;
    height: 38px;
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-surface-hover);
    border: 1px solid var(--color-border);
  }

  .provider-title-area {
    flex: 1;
  }

  .provider-title-area h3 {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-ink);
  }

  .provider-badge {
    display: inline-block;
    font-size: 0.6875rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 2px 6px;
    border-radius: 4px;
    margin-top: 2px;
  }

  .provider-badge.active {
    background: rgba(34, 197, 94, 0.15);
    color: #4ade80;
  }

  .provider-badge.inactive {
    background: rgba(255, 255, 255, 0.08);
    color: var(--color-ink-muted);
  }

  /* Toggle Switch */
  .toggle-switch {
    position: relative;
    display: inline-block;
    width: 44px;
    height: 24px;
  }

  .toggle-switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .toggle-slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--color-border);
    transition: 0.2s;
    border-radius: 24px;
  }

  .toggle-slider:before {
    position: absolute;
    content: "";
    height: 18px;
    width: 18px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: 0.2s;
    border-radius: 50%;
  }

  input:checked + .toggle-slider {
    background-color: var(--color-primary, #6366f1);
  }

  input:checked + .toggle-slider:before {
    transform: translateX(20px);
  }

  .provider-body {
    padding: var(--sp-5);
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }

  .instruction-box {
    background: rgba(99, 102, 241, 0.06);
    border: 1px solid rgba(99, 102, 241, 0.15);
    border-radius: var(--radius-md);
    padding: 12px;
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
  }

  .instruction-box ol {
    margin: 6px 0 0 16px;
    padding: 0;
    line-height: 1.5;
  }

  .instruction-box code {
    background: rgba(0, 0, 0, 0.3);
    padding: 2px 5px;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.75rem;
  }

  .copy-input-row {
    display: flex;
    gap: 8px;
  }

  .readonly-input {
    background: rgba(0, 0, 0, 0.25);
    font-family: monospace;
    font-size: 0.75rem;
    color: var(--color-ink-muted);
  }

  .secret-indicator {
    font-size: 0.6875rem;
    font-weight: 500;
    color: #4ade80;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: 8px;
  }

  .provider-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-top: 1px solid var(--color-border-subtle);
    background: rgba(255, 255, 255, 0.01);
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    padding: 60px 0;
    color: var(--color-ink-secondary);
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(255, 255, 255, 0.1);
    border-top-color: var(--color-primary, #6366f1);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
