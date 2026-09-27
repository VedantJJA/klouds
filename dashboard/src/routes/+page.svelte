<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Project, type Service, type Database } from '$lib/api/client';
  import { currentUser, userQuota } from '$lib/stores/auth';
  import {
    FolderKanban,
    Server,
    Database as DatabaseIcon,
    Plus,
    ArrowRight,
    ExternalLink,
    RefreshCw,
    Activity
  } from '@lucide/svelte';

  let projects = $state<Project[]>([]);
  let services = $state<Service[]>([]);
  let databases = $state<Database[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function loadDashboardData() {
    loading = true;
    error = '';
    try {
      const [p, s, d] = await Promise.all([
        api.listProjects(),
        api.listServices(),
        api.listDatabases()
      ]);
      projects = p || [];
      services = s || [];
      databases = d || [];
    } catch (err: any) {
      error = err.message || 'Failed to load platform data';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadDashboardData();
  });

  const runningServicesCount = $derived(services.filter(s => s.status === 'running').length);
</script>

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      <span>Platform</span>
      <span>/</span>
      <span class="text-white">Overview</span>
    </div>
    <h1 class="page-title">Dashboard</h1>
    <p class="page-subtitle">Platform health, resource utilization, and deployed applications</p>
  </div>

  <div class="header-actions">
    <button class="btn btn-secondary btn-sm" onclick={loadDashboardData} disabled={loading}>
      <RefreshCw size={14} class={loading ? 'spin' : ''} />
      <span>Refresh</span>
    </button>
    <a href="/projects" class="btn btn-primary btn-sm">
      <Plus size={14} />
      <span>New Project</span>
    </a>
  </div>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

<!-- Resource Usage & Stats Grid -->
<div class="metrics-grid">
  <div class="metric-card">
    <div class="metric-label">Active Projects</div>
    <div class="metric-value">{projects.length}</div>
    <div class="metric-sub">
      {#if $userQuota}
        Limit: {$userQuota.max_projects} projects
      {:else}
        Standard allocation
      {/if}
    </div>
  </div>

  <div class="metric-card">
    <div class="metric-label">Services Running</div>
    <div class="metric-value">{runningServicesCount} / {services.length}</div>
    <div class="metric-sub">
      {#if $userQuota}
        Quota: {$userQuota.max_services} services
      {:else}
        Active workloads
      {/if}
    </div>
  </div>

  <div class="metric-card">
    <div class="metric-label">Managed Databases</div>
    <div class="metric-value">{databases.length}</div>
    <div class="metric-sub">
      {#if $userQuota}
        Quota: {$userQuota.max_databases} instances
      {:else}
        PostgreSQL, Redis, MongoDB
      {/if}
    </div>
  </div>
</div>

<!-- Main Sections -->
<div class="dashboard-sections">
  <!-- Projects Summary -->
  <div class="card mb-4">
    <div class="card-header">
      <div class="flex items-center gap-2">
        <FolderKanban size={18} />
        <h3>Projects</h3>
      </div>
      <a href="/projects" class="btn btn-secondary btn-sm">
        <span>View All</span>
        <ArrowRight size={13} />
      </a>
    </div>

    {#if loading}
      <div class="p-4 text-center text-muted">Loading projects...</div>
    {:else if projects.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <FolderKanban size={32} />
        </div>
        <h3>No projects found</h3>
        <p class="text-sm text-muted mb-3">Create your first project to organize your services and databases.</p>
        <a href="/projects" class="btn btn-primary btn-sm">Create Project</a>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Project Name</th>
              <th>Description</th>
              <th>Created</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each projects.slice(0, 5) as project}
              <tr>
                <td>
                  <a href={`/projects/${project.id}`} class="project-name-link">
                    {project.name}
                  </a>
                </td>
                <td class="text-muted">{project.description || 'No description provided'}</td>
                <td class="font-mono text-xs">{new Date(project.created_at).toLocaleDateString()}</td>
                <td>
                  <a href={`/projects/${project.id}`} class="btn btn-secondary btn-sm">
                    Open
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Services Summary -->
  <div class="card">
    <div class="card-header">
      <div class="flex items-center gap-2">
        <Server size={18} />
        <h3>Recent Services</h3>
      </div>
      <a href="/projects" class="btn btn-secondary btn-sm">
        <span>Manage</span>
        <ArrowRight size={13} />
      </a>
    </div>

    {#if loading}
      <div class="p-4 text-center text-muted">Loading services...</div>
    {:else if services.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <Server size={32} />
        </div>
        <h3>No active services</h3>
        <p class="text-sm text-muted">Deploy a Git repository or Docker image from within a project.</p>
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
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each services.slice(0, 5) as service}
              <tr>
                <td>
                  <a href={`/services/${service.id}`} class="service-name-link">
                    {service.name}
                  </a>
                </td>
                <td>
                  <span class={`badge badge-${service.status}`}>
                    {service.status}
                  </span>
                </td>
                <td class="font-mono text-xs">{service.source_type}</td>
                <td>
                  {#if service.subdomain}
                    <span class="font-mono text-xs text-muted">{service.subdomain}</span>
                  {:else}
                    <span class="text-xs text-muted">Internal</span>
                  {/if}
                </td>
                <td>
                  <a href={`/services/${service.id}`} class="btn btn-secondary btn-sm">
                    Details
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

<style>
  .header-actions {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
  }

  .project-name-link,
  .service-name-link {
    font-weight: 600;
    color: var(--color-ink);
  }

  .project-name-link:hover,
  .service-name-link:hover {
    color: var(--color-accent);
    text-decoration: underline;
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
