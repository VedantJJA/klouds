<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Database } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Database as DatabaseIcon,
    Key,
    Copy,
    Check,
    Eye,
    EyeOff,
    Trash2,
    Server,
    ExternalLink
  } from '@lucide/svelte';

  const dbId = $derived($page.params.id || '');

  let database = $state<Database | null>(null);
  let connectionInfo = $state<{
    connection_url: string;
    internal_connection_string?: string;
    external_connection_string?: string;
    host: string;
    port: number;
    internal_host?: string;
    external_host?: string;
    external_port?: number;
    username?: string;
    password?: string;
    database?: string;
  } | null>(null);

  let showPassword = $state(false);
  let copied = $state(false);
  let loading = $state(true);
  let error = $state('');

  async function loadData() {
    if (!dbId) return;
    loading = true;
    error = '';
    try {
      const [d, conn] = await Promise.all([
        api.getDatabase(dbId),
        api.getDatabaseConnection(dbId).catch(() => null)
      ]);
      database = d;
      connectionInfo = conn;
    } catch (err: any) {
      error = err.message || 'Failed to load database details';
    } finally {
      loading = false;
    }
  }

  async function copyToClipboard(text: string) {
    if (!navigator.clipboard) return;
    await navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => {
      copied = false;
    }, 2000);
  }

  async function handleDelete() {
    if (!dbId) return;
    if (!confirm(`Are you sure you want to delete database "${database?.name}"? All data will be deleted.`)) {
      return;
    }

    try {
      await api.deleteDatabase(dbId);
      goto('/databases');
    } catch (err: any) {
      alert(err.message || 'Failed to delete database');
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Databases', href: '/databases' },
    { label: database?.name || 'Database Instance' }
  ]}
  backHref="/databases"
/>

{#if loading}
  <div class="card text-center p-5 text-muted">Loading database credentials...</div>
{:else if error}
  <div class="card text-danger p-4 mb-4">{error}</div>
{:else if database}
  <div class="page-header">
    <div>
      <div class="flex items-center gap-3 mb-1">
        <h1 class="page-title">{database.name}</h1>
        <span class={`badge badge-${database.status}`}>{database.status}</span>
      </div>
      <p class="page-subtitle">
        Engine: <span class="font-mono text-white uppercase">{database.engine} {database.version}</span>
      </p>
    </div>

    <button class="btn btn-danger btn-sm" onclick={handleDelete}>
      <Trash2 size={14} />
      <span>Delete Database</span>
    </button>
  </div>

  <div class="db-details-grid">
    <!-- Connection Details Card -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Key size={16} />
          <h3>Connection Parameters</h3>
        </div>
      </div>

      {#if connectionInfo}
        <div class="conn-rows">
          <div class="spec-row">
            <span class="spec-label">Host</span>
            <span class="spec-val font-mono">{connectionInfo.host}</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Port</span>
            <span class="spec-val font-mono">{connectionInfo.port}</span>
          </div>

          {#if connectionInfo.database}
            <div class="spec-row">
              <span class="spec-label">Database Name</span>
              <span class="spec-val font-mono">{connectionInfo.database}</span>
            </div>
          {/if}

          {#if connectionInfo.username}
            <div class="spec-row">
              <span class="spec-label">Username</span>
              <span class="spec-val font-mono">{connectionInfo.username}</span>
            </div>
          {/if}

          {#if connectionInfo.password}
            <div class="spec-row">
              <span class="spec-label">Password</span>
              <div class="flex items-center gap-2">
                <span class="spec-val font-mono">
                  {showPassword ? connectionInfo.password : '••••••••••••••••'}
                </span>
                <button
                  class="btn-icon"
                  onclick={() => showPassword = !showPassword}
                  title={showPassword ? 'Hide password' : 'Show password'}
                >
                  {#if showPassword}
                    <EyeOff size={14} />
                  {:else}
                    <Eye size={14} />
                  {/if}
                </button>
              </div>
            </div>
          {/if}

          {#if connectionInfo.connection_url || connectionInfo.internal_connection_string}
            <!-- Internal Connection String (Render-style) -->
            <div class="uri-box mt-3">
              <div class="flex items-center justify-between mb-1">
                <span class="uri-label">Internal Database URL</span>
                <span class="badge badge-success" style="font-size: 0.65rem;">Internal Network</span>
              </div>
              <p class="text-xs text-muted mb-2">For microservices & applications deployed inside your Klouds platform.</p>
              <div class="uri-content">
                <code class="uri-code">
                  {showPassword
                    ? (connectionInfo.internal_connection_string || connectionInfo.connection_url)
                    : (connectionInfo.internal_connection_string || connectionInfo.connection_url).replace(/:[^:@]+@/, ':••••••@')}
                </code>
                <button
                  class="btn btn-secondary btn-sm"
                  onclick={() => copyToClipboard(connectionInfo?.internal_connection_string || connectionInfo?.connection_url || '')}
                  title="Copy Internal URI"
                >
                  {#if copied}
                    <Check size={14} />
                    <span>Copied</span>
                  {:else}
                    <Copy size={14} />
                    <span>Copy</span>
                  {/if}
                </button>
              </div>
            </div>

            <!-- External Connection String (Render-style single port) -->
            {#if connectionInfo.external_connection_string}
              <div class="uri-box mt-3" style="border-color: rgba(59, 130, 246, 0.3); background: rgba(59, 130, 246, 0.03);">
                <div class="flex items-center justify-between mb-1">
                  <span class="uri-label text-accent">External Database URL</span>
                  <span class="badge badge-primary" style="font-size: 0.65rem;">Single-Port Router</span>
                </div>
                <p class="text-xs text-muted mb-2">Connect externally from psql, DBeaver, TablePlus, or local development on port {connectionInfo.external_port || 5432}.</p>
                <div class="uri-content">
                  <code class="uri-code">
                    {showPassword
                      ? connectionInfo.external_connection_string
                      : connectionInfo.external_connection_string.replace(/:[^:@]+@/, ':••••••@')}
                  </code>
                  <button
                    class="btn btn-secondary btn-sm"
                    onclick={() => copyToClipboard(connectionInfo?.external_connection_string || '')}
                    title="Copy External URI"
                  >
                    {#if copied}
                      <Check size={14} />
                      <span>Copied</span>
                    {:else}
                      <Copy size={14} />
                      <span>Copy</span>
                    {/if}
                  </button>
                </div>
              </div>
            {/if}
          {/if}
        </div>
      {:else}
        <p class="text-sm text-muted">Connection information is unavailable or the container is not initialized.</p>
      {/if}
    </div>

    <!-- Instance Specifications -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Server size={16} />
          <h3>Engine Specifications</h3>
        </div>
      </div>

      <div class="conn-rows">
        <div class="spec-row">
          <span class="spec-label">Engine Architecture</span>
          <span class="spec-val font-mono">{database.engine}</span>
        </div>

        <div class="spec-row">
          <span class="spec-label">Version Tag</span>
          <span class="spec-val font-mono">{database.version}</span>
        </div>

        <div class="spec-row">
          <span class="spec-label">Internal Port</span>
          <span class="spec-val font-mono">{database.port}</span>
        </div>

        <div class="spec-row">
          <span class="spec-label">Container ID</span>
          <span class="spec-val font-mono text-xs">
            {database.container_id ? database.container_id.slice(0, 12) : 'Unassigned'}
          </span>
        </div>

        <div class="spec-row">
          <span class="spec-label">Created At</span>
          <span class="spec-val text-xs text-muted">
            {new Date(database.created_at).toLocaleString()}
          </span>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .db-details-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: var(--sp-4);
  }

  .conn-rows {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .spec-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--color-border-subtle);
    padding-bottom: 8px;
    font-size: 0.8125rem;
  }

  .spec-row:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }

  .spec-label {
    color: var(--color-ink-muted);
  }

  .spec-val {
    color: var(--color-ink);
  }

  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 3px;
    border-radius: var(--radius-sm);
    color: var(--color-ink-muted);
    display: flex;
    align-items: center;
  }

  .btn-icon:hover {
    color: var(--color-ink);
  }

  .uri-box {
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--sp-3);
  }

  .uri-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-ink-muted);
    margin-bottom: 6px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .uri-content {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .uri-code {
    word-break: break-all;
    font-size: 0.75rem;
    color: var(--color-ink);
  }
</style>
