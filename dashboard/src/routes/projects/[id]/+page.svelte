<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Project, type Service, type Database } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    FolderKanban,
    Server,
    Database as DatabaseIcon,
    Plus,
    X,
    ExternalLink,
    Play,
    Square,
    RefreshCw,
    Trash2,
    Settings
  } from '@lucide/svelte';

  const projectId = $derived($page.params.id || '');

  let project = $state<Project | null>(null);
  let services = $state<Service[]>([]);
  let databases = $state<Database[]>([]);
  let activeTab = $state<'services' | 'databases' | 'settings'>('services');
  let loading = $state(true);
  let error = $state('');

  // Deploy Service Modal State
  let showDeployModal = $state(false);
  let svcName = $state('');
  let svcSourceType = $state<'git' | 'image'>('git');
  let svcGitRepo = $state('');
  let svcGitBranch = $state('main');
  let svcDockerImage = $state('');
  let svcPort = $state(80);
  let deploying = $state(false);
  let deployError = $state('');

  // Provision Database Modal State
  let showDbModal = $state(false);
  let dbName = $state('');
  let dbEngine = $state<'postgres' | 'redis' | 'mongodb' | 'mysql'>('postgres');
  let dbVersion = $state('16');
  let provisioning = $state(false);
  let provisionError = $state('');

  async function loadData() {
    if (!projectId) return;
    loading = true;
    error = '';
    try {
      const [p, s, d] = await Promise.all([
        api.getProject(projectId),
        api.listServices(projectId),
        api.listDatabases(projectId)
      ]);
      project = p;
      services = s || [];
      databases = d || [];
    } catch (err: any) {
      error = err.message || 'Failed to load project details';
    } finally {
      loading = false;
    }
  }

  async function handleDeployService(e: SubmitEvent) {
    e.preventDefault();
    if (!svcName.trim() || !projectId) return;

    deploying = true;
    deployError = '';
    try {
      const newSvc = await api.createService({
        project_id: projectId,
        name: svcName.trim(),
        source_type: svcSourceType,
        git_repo: svcSourceType === 'git' ? svcGitRepo.trim() : undefined,
        git_branch: svcSourceType === 'git' ? svcGitBranch.trim() : undefined,
        docker_image: svcSourceType === 'image' ? svcDockerImage.trim() : undefined,
        port: Number(svcPort)
      });
      services = [...services, newSvc];
      showDeployModal = false;
      svcName = '';
      svcGitRepo = '';
      svcDockerImage = '';
    } catch (err: any) {
      deployError = err.message || 'Failed to create service';
    } finally {
      deploying = false;
    }
  }

  async function handleProvisionDb(e: SubmitEvent) {
    e.preventDefault();
    if (!dbName.trim() || !projectId) return;

    provisioning = true;
    provisionError = '';
    try {
      const newDb = await api.createDatabase({
        project_id: projectId,
        name: dbName.trim(),
        engine: dbEngine,
        version: dbVersion
      });
      databases = [...databases, newDb];
      showDbModal = false;
      dbName = '';
    } catch (err: any) {
      provisionError = err.message || 'Failed to provision database';
    } finally {
      provisioning = false;
    }
  }

  async function handleDeleteProject() {
    if (!projectId) return;
    if (!confirm(`Are you sure you want to delete project "${project?.name}"? All associated services will be removed.`)) {
      return;
    }

    try {
      await api.deleteProject(projectId);
      goto('/projects');
    } catch (err: any) {
      alert(err.message || 'Failed to delete project');
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    { label: project?.name || 'Project Details' }
  ]}
  backHref="/projects"
/>

{#if loading}
  <div class="card text-center p-5 text-muted">Loading project details...</div>
{:else if error}
  <div class="card text-danger p-4 mb-4">{error}</div>
{:else if project}
  <div class="page-header">
    <div>
      <h1 class="page-title">{project.name}</h1>
      <p class="page-subtitle">{project.description || 'No description provided'}</p>
    </div>

    <div class="flex items-center gap-2">
      <button class="btn btn-secondary btn-sm" onclick={() => showDbModal = true}>
        <DatabaseIcon size={14} />
        <span>New Database</span>
      </button>

      <button class="btn btn-primary btn-sm" onclick={() => showDeployModal = true}>
        <Plus size={14} />
        <span>Deploy Service</span>
      </button>
    </div>
  </div>

  <!-- Tabs Navigation -->
  <div class="tabs-bar">
    <button
      class="tab-btn"
      class:active={activeTab === 'services'}
      onclick={() => activeTab = 'services'}
    >
      <Server size={14} />
      <span>Services ({services.length})</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'databases'}
      onclick={() => activeTab = 'databases'}
    >
      <DatabaseIcon size={14} />
      <span>Databases ({databases.length})</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'settings'}
      onclick={() => activeTab = 'settings'}
    >
      <Settings size={14} />
      <span>Settings</span>
    </button>
  </div>

  <!-- Tab 1: Services -->
  {#if activeTab === 'services'}
    {#if services.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <Server size={36} />
        </div>
        <h3>No Services Deployed</h3>
        <p class="text-sm text-muted mb-4">Deploy a web application, API, or worker into this project.</p>
        <button class="btn btn-primary btn-sm" onclick={() => showDeployModal = true}>
          <Plus size={14} />
          <span>Deploy First Service</span>
        </button>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Service</th>
              <th>Status</th>
              <th>Source</th>
              <th>Endpoint</th>
              <th>Port</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each services as svc}
              <tr>
                <td>
                  <a href={`/services/${svc.id}`} class="font-semibold text-white">
                    {svc.name}
                  </a>
                </td>
                <td>
                  <span class={`badge badge-${svc.status}`}>{svc.status}</span>
                </td>
                <td class="font-mono text-xs">{svc.source_type}</td>
                <td>
                  {#if svc.subdomain}
                    <span class="font-mono text-xs text-muted">{svc.subdomain}</span>
                  {:else}
                    <span class="text-xs text-muted">Internal</span>
                  {/if}
                </td>
                <td class="font-mono text-xs">{svc.port}</td>
                <td>
                  <a href={`/services/${svc.id}`} class="btn btn-secondary btn-sm">
                    Manage
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- Tab 2: Databases -->
  {#if activeTab === 'databases'}
    {#if databases.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <DatabaseIcon size={36} />
        </div>
        <h3>No Databases Provisioned</h3>
        <p class="text-sm text-muted mb-4">Provision high-performance PostgreSQL, Redis, MongoDB, or MySQL instances.</p>
        <button class="btn btn-primary btn-sm" onclick={() => showDbModal = true}>
          <DatabaseIcon size={14} />
          <span>Provision Database</span>
        </button>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Database</th>
              <th>Engine</th>
              <th>Version</th>
              <th>Port</th>
              <th>Status</th>
              <th>Action</th>
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
                <td class="font-mono text-xs">{db.engine}</td>
                <td class="font-mono text-xs">{db.version}</td>
                <td class="font-mono text-xs">{db.port}</td>
                <td>
                  <span class={`badge badge-${db.status}`}>{db.status}</span>
                </td>
                <td>
                  <a href={`/databases/${db.id}`} class="btn btn-secondary btn-sm">
                    Connection Info
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- Tab 3: Settings -->
  {#if activeTab === 'settings'}
    <div class="card">
      <div class="card-header">
        <h3>Danger Zone</h3>
      </div>
      <p class="text-sm text-muted mb-4">
        Deleting this project will permanently remove all associated service definitions, environment configurations, and managed database records.
      </p>
      <button class="btn btn-danger btn-sm" onclick={handleDeleteProject}>
        <Trash2 size={14} />
        <span>Delete Project</span>
      </button>
    </div>
  {/if}
{/if}

<!-- Deploy Service Modal -->
{#if showDeployModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => showDeployModal = false} aria-label="Close modal"></button>
    <div class="modal-content">
      <div class="modal-header">
        <h3>Deploy New Service</h3>
        <button class="btn-icon" onclick={() => showDeployModal = false}>
          <X size={16} />
        </button>
      </div>

      <form onsubmit={handleDeployService}>
        <div class="modal-body">
          {#if deployError}
            <div class="error-banner mb-3">
              <span>{deployError}</span>
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="svc-name">Service Name</label>
            <input
              id="svc-name"
              type="text"
              class="form-input"
              placeholder="e.g. web-app"
              bind:value={svcName}
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="svc-type">Source Type</label>
            <select id="svc-type" class="form-select" bind:value={svcSourceType}>
              <option value="git">Git Repository (Nixpacks Build)</option>
              <option value="image">Prebuilt Docker Image</option>
            </select>
          </div>

          {#if svcSourceType === 'git'}
            <div class="form-group">
              <label class="form-label" for="svc-repo">Git Repository URL</label>
              <input
                id="svc-repo"
                type="text"
                class="form-input"
                placeholder="https://github.com/example/repo"
                bind:value={svcGitRepo}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="svc-branch">Branch</label>
              <input
                id="svc-branch"
                type="text"
                class="form-input"
                placeholder="main"
                bind:value={svcGitBranch}
              />
            </div>
          {:else}
            <div class="form-group">
              <label class="form-label" for="svc-image">Docker Image</label>
              <input
                id="svc-image"
                type="text"
                class="form-input"
                placeholder="e.g. nginx:alpine"
                bind:value={svcDockerImage}
                required
              />
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="svc-port">Internal Port</label>
            <input
              id="svc-port"
              type="number"
              class="form-input"
              placeholder="80"
              bind:value={svcPort}
              required
            />
          </div>
        </div>

        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={() => showDeployModal = false}
          >
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={deploying}>
            {deploying ? 'Deploying...' : 'Deploy'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Provision Database Modal -->
{#if showDbModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => showDbModal = false} aria-label="Close modal"></button>
    <div class="modal-content">
      <div class="modal-header">
        <h3>Provision Managed Database</h3>
        <button class="btn-icon" onclick={() => showDbModal = false}>
          <X size={16} />
        </button>
      </div>

      <form onsubmit={handleProvisionDb}>
        <div class="modal-body">
          {#if provisionError}
            <div class="error-banner mb-3">
              <span>{provisionError}</span>
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="db-name">Database Instance Name</label>
            <input
              id="db-name"
              type="text"
              class="form-input"
              placeholder="e.g. primary-db"
              bind:value={dbName}
              required
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="db-engine">Database Engine</label>
            <select id="db-engine" class="form-select" bind:value={dbEngine}>
              <option value="postgres">PostgreSQL</option>
              <option value="redis">Redis</option>
              <option value="mongodb">MongoDB</option>
              <option value="mysql">MySQL</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="db-ver">Version</label>
            <input
              id="db-ver"
              type="text"
              class="form-input"
              placeholder="16"
              bind:value={dbVersion}
              required
            />
          </div>
        </div>

        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={() => showDbModal = false}
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
