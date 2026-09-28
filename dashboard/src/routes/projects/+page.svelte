<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import { FolderKanban, Plus, ArrowRight, Trash2, Calendar } from '@lucide/svelte';

  let projects = $state<Project[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function loadProjects() {
    loading = true;
    error = '';
    try {
      projects = await api.listProjects();
    } catch (err: any) {
      error = err.message || 'Failed to load projects';
    } finally {
      loading = false;
    }
  }

  async function handleDelete(projectId: string, name: string) {
    if (!confirm(`Are you sure you want to delete project "${name}"? This action cannot be undone.`)) {
      return;
    }

    try {
      await api.deleteProject(projectId);
      projects = projects.filter(p => p.id !== projectId);
    } catch (err: any) {
      alert(err.message || 'Failed to delete project');
    }
  }

  onMount(() => {
    loadProjects();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title">Projects</h1>
    <p class="page-subtitle">Organize and isolate your microservices, APIs, and managed databases</p>
  </div>

  <a href="/projects/new" class="btn btn-primary">
    <Plus size={15} />
    <span>New Project</span>
  </a>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

{#if loading}
  <div class="card text-center p-5 text-muted">Loading projects...</div>
{:else if projects.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon">
      <FolderKanban size={36} />
    </div>
    <h3>No Projects Found</h3>
    <p class="text-sm text-muted mb-4">You have not created any projects yet. Projects group your services and databases together.</p>
    <a href="/projects/new" class="btn btn-primary">
      <Plus size={15} />
      <span>Setup Your First Project</span>
    </a>
  </div>
{:else}
  <div class="projects-grid">
    {#each projects as project}
      <div class="card project-card">
        <div class="project-card-top">
          <div class="project-badge-icon">
            <FolderKanban size={18} />
          </div>
          <button
            class="btn-icon text-muted"
            title="Delete project"
            onclick={() => handleDelete(project.id, project.name)}
          >
            <Trash2 size={15} />
          </button>
        </div>

        <h3 class="project-title">
          <a href={`/projects/${project.id}`}>{project.name}</a>
        </h3>

        <p class="project-desc">
          {project.description || 'No description provided'}
        </p>

        <div class="project-footer">
          <div class="flex items-center gap-1 text-xs text-muted">
            <Calendar size={13} />
            <span>{new Date(project.created_at).toLocaleDateString()}</span>
          </div>

          <a href={`/projects/${project.id}`} class="btn btn-secondary btn-sm">
            <span>View Services</span>
            <ArrowRight size={13} />
          </a>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .projects-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: var(--sp-4);
  }

  .project-card {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    min-height: 180px;
  }

  .project-card-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--sp-3);
  }

  .project-badge-icon {
    width: 32px;
    height: 32px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink);
  }

  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .btn-icon:hover {
    color: var(--color-danger);
    background: var(--color-surface-subtle);
  }

  .project-title {
    margin-bottom: 6px;
  }

  .project-title a {
    color: var(--color-ink);
    text-decoration: none;
  }

  .project-title a:hover {
    text-decoration: underline;
  }

  .project-desc {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
    line-height: 1.5;
    flex: 1;
    margin-bottom: var(--sp-4);
  }

  .project-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid var(--color-border-subtle);
    padding-top: var(--sp-3);
  }
</style>
