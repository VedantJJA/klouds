<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type UserOAuthAccount, type OAuthProvider } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    GitBranch,
    Check,
    AlertCircle,
    Unlink,
    ExternalLink
  } from '@lucide/svelte';

  let accounts = $state<UserOAuthAccount[]>([]);
  let enabledProviders = $state<OAuthProvider[]>([]);
  let loading = $state(true);
  let disconnecting = $state<string | null>(null);
  let successMsg = $state('');
  let errorMsg = $state('');

  onMount(async () => {
    const searchParams = new URLSearchParams(window.location.search);
    const connectedParam = searchParams.get('connected');
    const errParam = searchParams.get('error');

    if (connectedParam) {
      successMsg = `Successfully connected your ${connectedParam.toUpperCase()} account! Your repositories will now appear automatically in the service creation wizard.`;
    }
    if (errParam) {
      errorMsg = decodeURIComponent(errParam);
    }

    await loadData();
  });

  async function loadData() {
    loading = true;
    try {
      const [accs, provs] = await Promise.all([
        api.getUserOAuthAccounts(),
        api.getOAuthProviders()
      ]);
      accounts = accs || [];
      enabledProviders = provs || [];
    } catch (err: any) {
      errorMsg = err.message || 'Failed to load connected accounts';
    } finally {
      loading = false;
    }
  }

  let ghAccount = $derived(accounts.find(a => a.provider === 'github'));
  let ghEnabled = $derived(enabledProviders.some(p => p.provider === 'github'));
  let glAccount = $derived(accounts.find(a => a.provider === 'gitlab'));
  let glEnabled = $derived(enabledProviders.some(p => p.provider === 'gitlab'));
  let bbAccount = $derived(accounts.find(a => a.provider === 'bitbucket'));
  let bbEnabled = $derived(enabledProviders.some(p => p.provider === 'bitbucket'));

  function connectAccount(provider: string) {
    window.location.href = `/api/user/oauth/${provider}/connect`;
  }

  async function disconnectAccount(provider: string) {
    if (!confirm(`Are you sure you want to disconnect your ${provider.toUpperCase()} account?`)) {
      return;
    }

    disconnecting = provider;
    try {
      await api.disconnectOAuthAccount(provider);
      successMsg = `Disconnected ${provider.toUpperCase()} account.`;
      await loadData();
    } catch (err: any) {
      errorMsg = err.message || `Failed to disconnect ${provider}`;
    } finally {
      disconnecting = null;
    }
  }
</script>

<div class="page-container">
  <Breadcrumbs
    items={[
      { label: 'Settings', href: '/settings' },
      { label: 'Connected Git Accounts', href: '/settings' }
    ]}
  />

  <div class="page-header">
    <div>
      <h1 class="page-title">Connected Git Accounts</h1>
      <p class="page-subtitle">
        Link your GitHub, GitLab, or Bitbucket accounts. Once connected, your repositories will automatically appear when creating or deploying services—no manual repository URLs or access tokens required.
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
      <p>Loading your connected accounts...</p>
    </div>
  {:else}
    <div class="accounts-grid">
      <!-- GITHUB -->
      <div class="account-card {ghAccount ? 'connected' : ''}">
        <div class="account-card-header">
          <div class="provider-badge github">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
            </svg>
          </div>
          <div class="account-title">
            <h3>GitHub</h3>
            <span class="status-indicator {ghAccount ? 'status-active' : 'status-inactive'}">
              {ghAccount ? 'Connected' : ghEnabled ? 'Ready to connect' : 'Admin config required'}
            </span>
          </div>
        </div>

        <div class="account-card-body">
          {#if ghAccount}
            <div class="profile-info">
              {#if ghAccount.avatar_url}
                <img src={ghAccount.avatar_url} alt={ghAccount.username} class="user-avatar" />
              {/if}
              <div class="profile-details">
                <span class="user-handle">@{ghAccount.username}</span>
                {#if ghAccount.email}
                  <span class="user-email">{ghAccount.email}</span>
                {/if}
              </div>
            </div>
            <p class="account-desc text-success">
              ✓ Klouds is authorized to read your public and private GitHub repositories.
            </p>
          {:else}
            <p class="account-desc">
              Connect your personal or organization GitHub account to import repositories and enable automated deployments.
            </p>
          {/if}
        </div>

        <div class="account-card-footer">
          {#if ghAccount}
            <button
              type="button"
              class="btn btn-outline-danger"
              disabled={disconnecting === 'github'}
              onclick={() => disconnectAccount('github')}
            >
              <Unlink size={16} />
              <span>{disconnecting === 'github' ? 'Disconnecting...' : 'Disconnect'}</span>
            </button>
          {:else if ghEnabled}
            <button
              type="button"
              class="btn btn-primary w-full"
              onclick={() => connectAccount('github')}
            >
              <GitBranch size={16} />
              <span>Connect GitHub</span>
            </button>
          {:else}
            <span class="text-muted text-xs">Platform admin must configure GitHub OAuth App first</span>
          {/if}
        </div>
      </div>

      <!-- GITLAB -->
      <div class="account-card {glAccount ? 'connected' : ''}">
        <div class="account-card-header">
          <div class="provider-badge gitlab">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="#fc6d26">
              <path d="M23.955 13.587l-1.342-4.135-2.664-8.189c-.135-.423-.73-.423-.867 0L16.418 9.45H7.582L4.918 1.263c-.136-.423-.731-.423-.867 0L1.387 9.452.045 13.587c-.121.375.014.789.331 1.023L12 23.054l11.624-8.444c.318-.234.453-.648.331-1.023z"/>
            </svg>
          </div>
          <div class="account-title">
            <h3>GitLab</h3>
            <span class="status-indicator {glAccount ? 'status-active' : 'status-inactive'}">
              {glAccount ? 'Connected' : glEnabled ? 'Ready to connect' : 'Admin config required'}
            </span>
          </div>
        </div>

        <div class="account-card-body">
          {#if glAccount}
            <div class="profile-info">
              {#if glAccount.avatar_url}
                <img src={glAccount.avatar_url} alt={glAccount.username} class="user-avatar" />
              {/if}
              <div class="profile-details">
                <span class="user-handle">@{glAccount.username}</span>
                {#if glAccount.email}
                  <span class="user-email">{glAccount.email}</span>
                {/if}
              </div>
            </div>
            <p class="account-desc text-success">
              ✓ Klouds is authorized to read your GitLab projects.
            </p>
          {:else}
            <p class="account-desc">
              Connect your GitLab.com or self-hosted GitLab account to deploy projects automatically.
            </p>
          {/if}
        </div>

        <div class="account-card-footer">
          {#if glAccount}
            <button
              type="button"
              class="btn btn-outline-danger"
              disabled={disconnecting === 'gitlab'}
              onclick={() => disconnectAccount('gitlab')}
            >
              <Unlink size={16} />
              <span>{disconnecting === 'gitlab' ? 'Disconnecting...' : 'Disconnect'}</span>
            </button>
          {:else if glEnabled}
            <button
              type="button"
              class="btn btn-primary w-full"
              onclick={() => connectAccount('gitlab')}
            >
              <GitBranch size={16} />
              <span>Connect GitLab</span>
            </button>
          {:else}
            <span class="text-muted text-xs">Platform admin must configure GitLab OAuth App first</span>
          {/if}
        </div>
      </div>

      <!-- BITBUCKET -->
      <div class="account-card {bbAccount ? 'connected' : ''}">
        <div class="account-card-header">
          <div class="provider-badge bitbucket">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="#0052cc">
              <path d="M.375 0h23.25c.207 0 .375.168.375.375v4.5a.375.375 0 01-.375.375h-7.662l-1.408 8.447a.375.375 0 01-.37.313H9.815a.375.375 0 01-.37-.313L8.037 5.25H.375A.375.375 0 010 4.875v-4.5C0 .168.168 0 .375 0z"/>
            </svg>
          </div>
          <div class="account-title">
            <h3>Bitbucket</h3>
            <span class="status-indicator {bbAccount ? 'status-active' : 'status-inactive'}">
              {bbAccount ? 'Connected' : bbEnabled ? 'Ready to connect' : 'Admin config required'}
            </span>
          </div>
        </div>

        <div class="account-card-body">
          {#if bbAccount}
            <div class="profile-info">
              {#if bbAccount.avatar_url}
                <img src={bbAccount.avatar_url} alt={bbAccount.username} class="user-avatar" />
              {/if}
              <div class="profile-details">
                <span class="user-handle">@{bbAccount.username}</span>
                {#if bbAccount.email}
                  <span class="user-email">{bbAccount.email}</span>
                {/if}
              </div>
            </div>
            <p class="account-desc text-success">
              ✓ Klouds is authorized to read your Bitbucket repositories.
            </p>
          {:else}
            <p class="account-desc">
              Connect your Bitbucket account to list repositories and build services effortlessly.
            </p>
          {/if}
        </div>

        <div class="account-card-footer">
          {#if bbAccount}
            <button
              type="button"
              class="btn btn-outline-danger"
              disabled={disconnecting === 'bitbucket'}
              onclick={() => disconnectAccount('bitbucket')}
            >
              <Unlink size={16} />
              <span>{disconnecting === 'bitbucket' ? 'Disconnecting...' : 'Disconnect'}</span>
            </button>
          {:else if bbEnabled}
            <button
              type="button"
              class="btn btn-primary w-full"
              onclick={() => connectAccount('bitbucket')}
            >
              <GitBranch size={16} />
              <span>Connect Bitbucket</span>
            </button>
          {:else}
            <span class="text-muted text-xs">Platform admin must configure Bitbucket OAuth App first</span>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-container {
    max-width: 1000px;
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

  .accounts-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: var(--sp-5);
  }

  .account-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: all var(--transition-fast);
  }

  .account-card.connected {
    border-color: rgba(99, 102, 241, 0.4);
    box-shadow: 0 4px 20px rgba(99, 102, 241, 0.08);
  }

  .account-card-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .provider-badge {
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-surface-hover);
    border: 1px solid var(--color-border);
  }

  .account-title {
    flex: 1;
  }

  .account-title h3 {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-ink);
  }

  .status-indicator {
    font-size: 0.75rem;
    display: inline-block;
  }

  .status-active {
    color: #4ade80;
    font-weight: 600;
  }

  .status-inactive {
    color: var(--color-ink-muted);
  }

  .account-card-body {
    padding: var(--sp-5);
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .profile-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .user-avatar {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    border: 1px solid var(--color-border);
  }

  .profile-details {
    display: flex;
    flex-direction: column;
  }

  .user-handle {
    font-weight: 600;
    font-size: 0.9375rem;
    color: var(--color-ink);
  }

  .user-email {
    font-size: 0.75rem;
    color: var(--color-ink-secondary);
  }

  .account-desc {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
    line-height: 1.5;
  }

  .text-success {
    color: #4ade80;
  }

  .account-card-footer {
    padding: var(--sp-4) var(--sp-5);
    border-top: 1px solid var(--color-border-subtle);
    background: rgba(255, 255, 255, 0.01);
    display: flex;
    align-items: center;
    justify-content: flex-end;
  }

  .btn-outline-danger {
    background: transparent;
    border: 1px solid rgba(239, 68, 68, 0.4);
    color: #f87171;
    padding: 6px 12px;
    border-radius: var(--radius-md);
    font-size: 0.8125rem;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .btn-outline-danger:hover {
    background: rgba(239, 68, 68, 0.1);
    border-color: #f87171;
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
