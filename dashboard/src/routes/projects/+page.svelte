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
  <div class="card p-0">
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Project Name</th>
            <th>Description</th>
            <th>Slug</th>
            <th>Created</th>
            <th style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each projects as project}
            <tr>
              <td>
                <div class="flex items-center gap-2">
                  <div class="project-list-icon">
                    <FolderKanban size={16} />
                  </div>
                  <a href={`/projects/${project.id}`} class="project-name-link font-semibold">
                    {project.name}
                  </a>
                </div>
              </td>
              <td class="text-muted text-sm">{project.description || 'No description provided'}</td>
              <td><span class="font-mono text-xs text-muted">{project.slug}</span></td>
              <td class="font-mono text-xs text-muted">{new Date(project.created_at).toLocaleDateString()}</td>
              <td style="text-align: right;">
                <div class="flex items-center justify-end gap-2">
                  <a href={`/projects/${project.id}`} class="btn btn-secondary btn-sm">
                    <span>View Project</span>
                    <ArrowRight size={13} />
                  </a>
                  <button
                    class="btn btn-danger btn-sm"
                    title="Delete project"
                    onclick={() => handleDelete(project.id, project.name)}
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
  </div>
{/if}

<style>
  .project-list-icon {
    width: 28px;
    height: 28px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink);
    flex-shrink: 0;
  }

  .project-name-link {
    color: var(--color-ink);
    text-decoration: none;
    font-size: 0.875rem;
    transition: color var(--transition-fast);
  }

  .project-name-link:hover {
    color: var(--color-accent);
    text-decoration: underline;
  }
</style>
