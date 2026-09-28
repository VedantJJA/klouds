<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Project, type Service, type Database, type DetectionResult } from '$lib/api/client';
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
    Settings,
    Layers,
    GitBranch,
    Sparkles,
    CheckCircle2
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
  let svcRootDir = $state('.');
  let svcDockerImage = $state('');
  let svcPort = $state(80);
  let deploying = $state(false);
  let deployError = $state('');

  // Blueprint / Monorepo Modal State
  let showBlueprintModal = $state(false);
  let bpMode = $state<'repo' | 'yaml'>('repo');
  let bpRepoUrl = $state('');
  let bpBranch = $state('');
  let bpYaml = $state('');
  let bpScanning = $state(false);
  let bpApplying = $state(false);
  let bpError = $state('');
  let bpSuccess = $state('');
  let detectedResult = $state<DetectionResult | null>(null);

  // Provision Database Modal State
  let showDbModal = $state(false);
  let dbName = $state('');
  let dbEngine = $state<'postgresql' | 'postgres' | 'redis' | 'mongodb' | 'mysql'>('postgresql');
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
        root_dir: svcSourceType === 'git' ? (svcRootDir.trim() || '.') : undefined,
        docker_image: svcSourceType === 'image' ? svcDockerImage.trim() : undefined,
        port: Number(svcPort)
      });
      services = [...services, newSvc];
      showDeployModal = false;
      svcName = '';
      svcGitRepo = '';
      svcRootDir = '.';
      svcDockerImage = '';
    } catch (err: any) {
      deployError = err.message || 'Failed to create service';
    } finally {
      deploying = false;
    }
  }

  async function handleScanRepo() {
    if (!bpRepoUrl.trim() || !projectId) return;
    bpScanning = true;
    bpError = '';
    bpSuccess = '';
    detectedResult = null;
    try {
      const res = await api.detectBlueprint(projectId, bpRepoUrl.trim(), bpBranch.trim() || undefined);
      detectedResult = res;
      if (res.detected_branch) {
        bpBranch = res.detected_branch;
      }
    } catch (err: any) {
      bpError = err.message || 'Failed to detect services in repository';
    } finally {
      bpScanning = false;
    }
  }

  async function handleApplyBlueprint() {
    if (!projectId) return;
    bpApplying = true;
    bpError = '';
    bpSuccess = '';
    try {
      let payload: { repo_url?: string; branch?: string; yaml_content?: string } = {};
      if (bpMode === 'repo') {
        payload = { repo_url: bpRepoUrl.trim(), branch: bpBranch.trim() || detectedResult?.detected_branch || '' };
      } else {
        payload = { yaml_content: bpYaml.trim() };
      }
      const res = await api.applyBlueprint(projectId, payload);
      bpSuccess = `Successfully deployed! Created ${res.result.services_created.length} service(s), updated ${res.result.services_updated.length}.`;
      await loadData();
      setTimeout(() => {
        showBlueprintModal = false;
        bpSuccess = '';
        detectedResult = null;
      }, 1800);
    } catch (err: any) {
      bpError = err.message || 'Failed to apply blueprint';
    } finally {
      bpApplying = false;
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
    if (!confirm(`Are you sure you want to delete "${project?.name}"? All associated services and databases will be permanently removed.`)) {
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

<svelte:head>
  <title>{project?.name || 'Project'} | Klouds</title>
</svelte:head>

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
      <button class="btn btn-secondary btn-sm" onclick={() => showBlueprintModal = true}>
        <Layers size={14} />
        <span>Deploy Blueprint</span>
      </button>

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
        <p class="text-sm text-muted mb-4">Deploy multiple services from a single GitHub repo or deploy a single app.</p>
        <div class="flex gap-2">
          <button class="btn btn-secondary btn-sm" onclick={() => showBlueprintModal = true}>
            <Layers size={14} />
            <span>Deploy Blueprint / Monorepo</span>
          </button>
          <button class="btn btn-primary btn-sm" onclick={() => showDeployModal = true}>
            <Plus size={14} />
            <span>Deploy Single Service</span>
          </button>
        </div>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Service</th>
              <th>Status</th>
              <th>Source / Subdirectory</th>
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
                <td>
                  <span class="font-mono text-xs">{svc.source_type}</span>
                  {#if svc.root_dir && svc.root_dir !== '.'}
                    <span class="badge badge-secondary ml-1 font-mono text-xs">{svc.root_dir}</span>
                  {/if}
                </td>
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
                    View
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
        <p class="text-sm text-muted mb-4">Attach managed PostgreSQL, Redis, MongoDB, or MySQL instances.</p>
        <button class="btn btn-primary btn-sm" onclick={() => showDbModal = true}>
          <Plus size={14} />
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
              <th>Status</th>
              <th>Port</th>
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
                <td>
                  <span class="badge badge-secondary">{db.engine} {db.version}</span>
                </td>
                <td>
                  <span class={`badge badge-${db.status}`}>{db.status}</span>
                </td>
                <td class="font-mono text-xs">{db.port}</td>
                <td>
                  <a href={`/databases/${db.id}`} class="btn btn-secondary btn-sm">
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

  <!-- Tab 3: Settings -->
  {#if activeTab === 'settings'}
    <div class="card p-5 max-w-2xl">
      <div class="mb-4">
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

<!-- Deploy Blueprint / Monorepo Modal -->
{#if showBlueprintModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => showBlueprintModal = false} aria-label="Close modal"></button>
    <div class="modal-content max-w-2xl">
      <div class="modal-header">
        <div class="flex items-center gap-2">
          <Layers size={18} class="text-primary" />
          <h3>Deploy Blueprint / Monorepo</h3>
        </div>
        <button class="btn-icon" onclick={() => showBlueprintModal = false}>
          <X size={16} />
        </button>
      </div>

      <div class="modal-body">
        <p class="text-xs text-muted mb-4">
          Host multiple services and databases from a single Git repository using automated detection or declarative Infrastructure-as-Code (<code>klouds.yaml</code> or <code>render.yaml</code>).
        </p>

        <div class="tabs-bar mb-4">
          <button
            type="button"
            class="tab-btn"
            class:active={bpMode === 'repo'}
            onclick={() => bpMode = 'repo'}
          >
            <GitBranch size={14} />
            <span>Git Repository Auto-Detection</span>
          </button>
          <button
            type="button"
            class="tab-btn"
            class:active={bpMode === 'yaml'}
            onclick={() => bpMode = 'yaml'}
          >
            <Layers size={14} />
            <span>YAML Blueprint</span>
          </button>
        </div>

        {#if bpError}
          <div class="error-banner mb-3">
            <span>{bpError}</span>
          </div>
        {/if}

        {#if bpSuccess}
          <div class="success-banner mb-3">
            <CheckCircle2 size={14} />
            <span>{bpSuccess}</span>
          </div>
        {/if}

        {#if bpMode === 'repo'}
          <div class="form-group">
            <label class="form-label" for="bp-repo">Git Repository URL</label>
            <div class="flex gap-2">
              <input
                id="bp-repo"
                type="text"
                class="form-input flex-1"
                placeholder="https://github.com/org/monorepo"
                bind:value={bpRepoUrl}
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={handleScanRepo}
                disabled={bpScanning || !bpRepoUrl.trim()}
              >
                <Sparkles size={14} />
                <span>{bpScanning ? 'Scanning...' : 'Scan & Auto-Detect'}</span>
              </button>
            </div>
            <span class="text-xs text-muted">Supports monorepos with frontend/backend subdirectories, docker-compose, or blueprints.</span>
          </div>

          <div class="form-group">
            <label class="form-label" for="bp-branch">Branch</label>
            <input
              id="bp-branch"
              type="text"
              class="form-input"
              placeholder="master, main, or auto-detect"
              bind:value={bpBranch}
            />
          </div>

          {#if detectedResult}
            <div class="detected-box mt-4">
              <div class="detected-header">
                <span class="text-xs font-semibold uppercase text-muted">
                  Detected ({detectedResult.source}): {detectedResult.blueprint.services.length} Service(s)
                </span>
              </div>
              <div class="detected-list">
                {#each detectedResult.blueprint.services as s}
                  <div class="detected-item">
                    <div>
                      <div class="font-semibold text-white text-sm">{s.name}</div>
                      <div class="text-xs text-muted font-mono">
                        rootDir: {s.root_dir || '.'} | port: {s.port || 3000}
                      </div>
                    </div>
                    <div class="flex items-center gap-1">
                      <span class="badge badge-secondary font-mono text-xs">{s.env || 'auto'}</span>
                      <span class="badge badge-primary font-mono text-xs">{s.type || 'web'}</span>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        {:else}
          <div class="form-group">
            <label class="form-label" for="bp-yaml">YAML Specification</label>
            <textarea
              id="bp-yaml"
              class="form-input font-mono text-xs"
              rows={12}
              placeholder="services:&#10;  - name: api&#10;    type: web&#10;    env: go&#10;    rootDir: backend&#10;  - name: web&#10;    type: web&#10;    env: node&#10;    rootDir: frontend"
              bind:value={bpYaml}
            ></textarea>
            <span class="text-xs text-muted">Supports both <code>klouds.yaml</code> and <code>render.yaml</code> syntax.</span>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button
          type="button"
          class="btn btn-secondary"
          onclick={() => showBlueprintModal = false}
        >
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-primary"
          onclick={handleApplyBlueprint}
          disabled={bpApplying || (bpMode === 'repo' && !bpRepoUrl.trim()) || (bpMode === 'yaml' && !bpYaml.trim())}
        >
          {bpApplying ? 'Deploying Stack...' : 'Deploy Stack'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Deploy Single Service Modal -->
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
              <label class="form-label" for="svc-root-dir">Root Directory (Optional)</label>
              <input
                id="svc-root-dir"
                type="text"
                class="form-input"
                placeholder="e.g. backend or apps/web (default: .)"
                bind:value={svcRootDir}
              />
              <span class="text-xs text-muted">Subdirectory path for monorepos</span>
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
            {deploying ? 'Deploying...' : 'Deploy Service'}
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
              <option value="postgresql">PostgreSQL</option>
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

  .success-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: rgba(34, 197, 94, 0.15);
    border: 1px solid rgba(34, 197, 94, 0.3);
    border-radius: var(--radius-md);
    color: #4ade80;
    font-size: 0.8125rem;
  }

  .detected-box {
    background: var(--color-bg-base);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    padding: 12px;
  }

  .detected-header {
    margin-bottom: 8px;
    padding-bottom: 6px;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .detected-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .detected-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    background: var(--color-bg-surface);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-sm);
  }
</style>
