<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Service, type Deployment, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Server,
    Play,
    Square,
    RotateCw,
    Trash2,
    ExternalLink,
    GitBranch,
    Clock,
    Terminal,
    Cpu,
    HardDrive,
    Layers,
    FileText
  } from '@lucide/svelte';

  const serviceId = $derived($page.params.id || '');

  let service = $state<Service | null>(null);
  let parentProject = $state<Project | null>(null);
  let deployments = $state<Deployment[]>([]);
  let activeTab = $state<'overview' | 'deployments' | 'logs' | 'settings'>('overview');
  let selectedDeployment = $state<Deployment | null>(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let error = $state('');
  let pollTimer: any = null;

  async function loadServiceData(silent = false) {
    if (!serviceId) return;
    if (!silent) loading = true;
    error = '';
    try {
      const svc = await api.getService(serviceId);
      service = svc;

      if (svc.project_id) {
        try {
          parentProject = await api.getProject(svc.project_id);
        } catch {
          // Non-critical if project fetch fails
        }
      }

      const deps = await api.getDeployments(serviceId);
      deployments = deps || [];
      if (deployments.length > 0) {
        if (!selectedDeployment) {
          selectedDeployment = deployments[0];
        } else {
          const updated = deployments.find(d => d.id === selectedDeployment.id);
          selectedDeployment = updated || deployments[0];
        }
      }

      // Check if build is ongoing to maintain polling
      const isBuilding = deployments.some(d => d.status === 'building' || d.status === 'deploying' || d.status === 'queued');
      if (isBuilding && !pollTimer) {
        pollTimer = setInterval(() => {
          loadServiceData(true);
        }, 2000);
      } else if (!isBuilding && pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    } catch (err: any) {
      if (!silent) error = err.message || 'Failed to load service details';
    } finally {
      if (!silent) loading = false;
    }
  }

  async function handleDeploy() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.deployService(serviceId);
      activeTab = 'logs';
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to trigger deployment');
    } finally {
      actionLoading = false;
    }
  }

  async function handleRestart() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.restartService(serviceId);
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to restart service');
    } finally {
      actionLoading = false;
    }
  }

  async function handleStop() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.stopService(serviceId);
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to stop service');
    } finally {
      actionLoading = false;
    }
  }

  async function handleDelete() {
    if (!serviceId) return;
    if (!confirm(`Are you sure you want to delete service "${service?.name}"?`)) {
      return;
    }

    try {
      await api.deleteService(serviceId);
      if (service?.project_id) {
        goto(`/projects/${service.project_id}`);
      } else {
        goto('/projects');
      }
    } catch (err: any) {
      alert(err.message || 'Failed to delete service');
    }
  }

  onMount(() => {
    loadServiceData();
  });

  onDestroy(() => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    ...(parentProject ? [{ label: parentProject.name, href: `/projects/${parentProject.id}` }] : []),
    { label: service?.name || 'Service Details' }
  ]}
  backHref={parentProject ? `/projects/${parentProject.id}` : '/projects'}
/>

{#if loading}
  <div class="card text-center p-5 text-muted">Loading service specifications...</div>
{:else if error}
  <div class="card text-danger p-4 mb-4">{error}</div>
{:else if service}
  <div class="page-header">
    <div>
      <div class="flex items-center gap-3 mb-1">
        <h1 class="page-title">{service.name}</h1>
        <span class={`badge badge-${service.status}`}>{service.status}</span>
      </div>

      {#if service.subdomain}
        <div class="service-domain flex items-center gap-1 text-sm text-muted">
          <span>Public URL:</span>
          <span class="font-mono text-white">{service.subdomain}</span>
        </div>
      {/if}
    </div>

    <div class="flex items-center gap-2">
      <button
        class="btn btn-primary btn-sm"
        onclick={handleDeploy}
        disabled={actionLoading}
        title="Deploy latest code"
      >
        <Play size={14} />
        <span>Deploy</span>
      </button>

      <button
        class="btn btn-secondary btn-sm"
        onclick={handleRestart}
        disabled={actionLoading}
        title="Restart container"
      >
        <RotateCw size={14} class={actionLoading ? 'spin' : ''} />
        <span>Restart</span>
      </button>

      {#if service.status === 'running'}
        <button
          class="btn btn-secondary btn-sm"
          onclick={handleStop}
          disabled={actionLoading}
          title="Stop container"
        >
          <Square size={14} />
          <span>Stop</span>
        </button>
      {/if}

      <button
        class="btn btn-danger btn-sm"
        onclick={handleDelete}
        title="Delete service"
      >
        <Trash2 size={14} />
        <span>Delete</span>
      </button>
    </div>
  </div>

  <!-- Tabs Navigation -->
  <div class="tabs-bar">
    <button
      class="tab-btn"
      class:active={activeTab === 'overview'}
      onclick={() => activeTab = 'overview'}
    >
      <Layers size={14} />
      <span>Overview</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'deployments'}
      onclick={() => activeTab = 'deployments'}
    >
      <Clock size={14} />
      <span>Deployments ({deployments.length})</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'logs'}
      onclick={() => activeTab = 'logs'}
    >
      <Terminal size={14} />
      <span>Build Logs</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'settings'}
      onclick={() => activeTab = 'settings'}
    >
      <span>Settings</span>
    </button>
  </div>

  <!-- Tab 1: Overview -->
  {#if activeTab === 'overview'}
    <div class="overview-grid">
      <div class="card">
        <div class="card-header">
          <h3>Source Configuration</h3>
        </div>

        <div class="spec-list">
          <div class="spec-row">
            <span class="spec-label">Source Type</span>
            <span class="spec-val font-mono">{service.source_type}</span>
          </div>

          {#if service.git_repo}
            <div class="spec-row">
              <span class="spec-label">Git Repository</span>
              <span class="spec-val font-mono truncate">{service.git_repo}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Branch</span>
              <span class="spec-val font-mono">{service.git_branch || 'main'}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Root Directory</span>
              <span class="spec-val font-mono">{service.root_dir || '.'}</span>
            </div>
          {/if}

          {#if service.docker_image}
            <div class="spec-row">
              <span class="spec-label">Container Image</span>
              <span class="spec-val font-mono truncate">{service.docker_image}</span>
            </div>
          {/if}

          <div class="spec-row">
            <span class="spec-label">Port Allocation</span>
            <span class="spec-val font-mono">{service.port}</span>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3>Resource Limits</h3>
        </div>

        <div class="spec-list">
          <div class="spec-row">
            <span class="spec-label">CPU Allotment</span>
            <span class="spec-val font-mono">{service.cpu_limit || 1000} mCPU</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Memory Limit</span>
            <span class="spec-val font-mono">{service.memory_limit || 512} MB</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Container ID</span>
            <span class="spec-val font-mono text-xs truncate">
              {service.container_id ? service.container_id.slice(0, 12) : 'Unassigned'}
            </span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Created At</span>
            <span class="spec-val text-xs text-muted">
              {new Date(service.created_at).toLocaleString()}
            </span>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tab 2: Deployments -->
  {#if activeTab === 'deployments'}
    {#if deployments.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <Clock size={36} />
        </div>
        <h3>No Deployments Recorded</h3>
        <p class="text-sm text-muted">Deployments will appear here once the build engine triggers a commit build or deployment.</p>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Commit</th>
              <th>Message</th>
              <th>Started</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each deployments as dep}
              <tr>
                <td>
                  <span class={`badge badge-${dep.status}`}>{dep.status}</span>
                </td>
                <td class="font-mono text-xs">
                  {dep.commit_hash ? dep.commit_hash.slice(0, 7) : 'Manual'}
                </td>
                <td class="text-muted truncate" style="max-width: 250px;">
                  {dep.commit_message || 'Deployment triggered via dashboard'}
                </td>
                <td class="font-mono text-xs">{new Date(dep.created_at).toLocaleString()}</td>
                <td>
                  <button
                    class="btn btn-secondary btn-sm"
                    onclick={() => {
                      selectedDeployment = dep;
                      activeTab = 'logs';
                    }}
                  >
                    View Logs
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- Tab 3: Build Logs -->
  {#if activeTab === 'logs'}
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Terminal size={16} />
          <h3>Build & Execution Logs</h3>
        </div>
        {#if selectedDeployment}
          <span class="font-mono text-xs text-muted">
            Deployment: {selectedDeployment.id.slice(0, 8)}
          </span>
        {/if}
      </div>

      <div class="log-viewer">
        {#if selectedDeployment && (selectedDeployment.build_log || selectedDeployment.build_logs)}
          {#each (selectedDeployment.build_log || selectedDeployment.build_logs || '').split('\n') as line}
            <div class="log-line-stdout">{line}</div>
          {/each}
        {:else if selectedDeployment && (selectedDeployment.status === 'building' || selectedDeployment.status === 'deploying')}
          <div class="log-line-build">[klouds-builder] Build in progress... Streaming logs will appear as build executes.</div>
        {:else}
          <div class="log-line-system">No build logs available for this deployment.</div>
          {#if selectedDeployment}
            <div class="log-line-stdout">Deployment status: {selectedDeployment.status}</div>
          {/if}
        {/if}
      </div>
    </div>
  {/if}

  <!-- Tab 4: Settings -->
  {#if activeTab === 'settings'}
    <div class="card">
      <div class="card-header">
        <h3>Service Destruction</h3>
      </div>
      <p class="text-sm text-muted mb-4">
        Deleting this service stops and removes the running Docker container and detaches all Caddy routing rules.
      </p>
      <button class="btn btn-danger btn-sm" onclick={handleDelete}>
        <Trash2 size={14} />
        <span>Delete Service</span>
      </button>
    </div>
  {/if}
{/if}

<style>
  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: var(--sp-4);
  }

  .spec-list {
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

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
