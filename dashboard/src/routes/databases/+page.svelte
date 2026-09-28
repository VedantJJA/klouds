<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Database, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import { Database as DatabaseIcon, Plus, X, Trash2, Key, Server } from '@lucide/svelte';

  let databases = $state<Database[]>([]);
  let projects = $state<Project[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Provision Modal State
  let showModal = $state(false);
  let selectedProjectId = $state('');
  let name = $state('');
  let engine = $state<'postgresql' | 'postgres' | 'redis' | 'mongodb' | 'mysql'>('postgresql');
  let version = $state('16');
  let provisioning = $state(false);
  let provisionError = $state('');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [d, p] = await Promise.all([
        api.listDatabases(),
        api.listProjects()
      ]);
      databases = d || [];
      projects = p || [];
      if (projects.length > 0 && !selectedProjectId) {
        selectedProjectId = projects[0].id;
      }
    } catch (err: any) {
      error = err.message || 'Failed to load databases';
    } finally {
      loading = false;
    }
  }

  async function handleProvision(e: SubmitEvent) {
    e.preventDefault();
    if (!name.trim() || !selectedProjectId) return;

    provisioning = true;
    provisionError = '';
    try {
      const created = await api.createDatabase({
        project_id: selectedProjectId,
        name: name.trim(),
        engine,
        version
      });
      databases = [created, ...databases];
      name = '';
      showModal = false;
    } catch (err: any) {
      provisionError = err.message || 'Failed to provision database';
    } finally {
      provisioning = false;
    }
  }

  async function handleDelete(id: string, dbName: string) {
    if (!confirm(`Are you sure you want to delete database "${dbName}"? All data in the instance will be deleted.`)) {
      return;
    }

    try {
      await api.deleteDatabase(id);
      databases = databases.filter(d => d.id !== id);
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
    { label: 'Databases' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title">Managed Databases</h1>
    <p class="page-subtitle">Dedicated database engines deployed in isolated containers with automated volume persistence</p>
  </div>

  <button class="btn btn-primary" onclick={() => showModal = true}>
    <Plus size={15} />
    <span>Provision Database</span>
  </button>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

{#if loading}
  <div class="card text-center p-5 text-muted">Loading database engines...</div>
{:else if databases.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon">
      <DatabaseIcon size={36} />
    </div>
    <h3>No Managed Databases Found</h3>
    <p class="text-sm text-muted mb-4">Provision a dedicated PostgreSQL, Redis, MongoDB, or MySQL container for your projects.</p>
    <button class="btn btn-primary" onclick={() => showModal = true}>
      <Plus size={15} />
      <span>Provision First Database</span>
    </button>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Instance Name</th>
          <th>Engine</th>
          <th>Version</th>
          <th>Port</th>
          <th>Status</th>
          <th>Created</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each databases as db}
          <tr>
            <td>
              <a href={`/databases/${db.id}`} class="font-semibold text-white">
                {db.name}
              </a>
            </td>
            <td class="font-mono text-xs uppercase">{db.engine}</td>
            <td class="font-mono text-xs">{db.version}</td>
            <td class="font-mono text-xs">{db.port}</td>
            <td>
              <span class={`badge badge-${db.status}`}>{db.status}</span>
            </td>
            <td class="font-mono text-xs text-muted">
              {new Date(db.created_at).toLocaleDateString()}
            </td>
            <td>
              <div class="flex items-center gap-2">
                <a href={`/databases/${db.id}`} class="btn btn-secondary btn-sm">
                  <Key size={13} />
                  <span>Connection</span>
                </a>
                <button
                  class="btn btn-danger btn-sm"
                  onclick={() => handleDelete(db.id, db.name)}
                  title="Delete database"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<!-- Provision Database Modal -->
{#if showModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => showModal = false} aria-label="Close modal"></button>
    <div class="modal-content">
      <div class="modal-header">
        <h3>Provision Managed Database</h3>
        <button class="btn-icon" onclick={() => showModal = false}>
          <X size={16} />
        </button>
      </div>

      <form onsubmit={handleProvision}>
        <div class="modal-body">
          {#if provisionError}
            <div class="error-banner mb-3">
              <span>{provisionError}</span>
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="db-proj">Target Project</label>
            <select id="db-proj" class="form-select" bind:value={selectedProjectId} required>
              {#each projects as proj}
                <option value={proj.id}>{proj.name}</option>
              {/each}
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="db-name-input">Database Name</label>
            <input
              id="db-name-input"
              type="text"
              class="form-input"
              placeholder="e.g. main-db"
              bind:value={name}
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="db-engine-select">Database Engine</label>
            <select id="db-engine-select" class="form-select" bind:value={engine}>
              <option value="postgresql">PostgreSQL</option>
              <option value="redis">Redis</option>
              <option value="mongodb">MongoDB</option>
              <option value="mysql">MySQL</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="db-ver-input">Version</label>
            <input
              id="db-ver-input"
              type="text"
              class="form-input"
              placeholder="16"
              bind:value={version}
              required
            />
          </div>
        </div>

        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={() => showModal = false}
          >
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={provisioning}>
            {provisioning ? 'Provisioning...' : 'Provision'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink-muted);
  }

  .btn-icon:hover {
    color: var(--color-ink);
  }

  .error-banner {
    padding: 8px 12px;
    background: var(--color-danger-subtle);
    border: 1px solid rgba(248, 113, 113, 0.3);
    border-radius: var(--radius-md);
    color: var(--color-danger);
    font-size: 0.8125rem;
  }
</style>
