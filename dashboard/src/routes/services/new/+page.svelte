<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import { Server, FolderKanban, ArrowRight, Plus } from '@lucide/svelte';

  let projects = $state<Project[]>([]);
  let loading = $state(true);

  onMount(async () => {
    const qProject = $page.url.searchParams.get('project');
    if (qProject) {
      goto(`/projects/${qProject}/services/new`);
      return;
    }

    try {
      projects = await api.listProjects();
      if (projects.length === 1) {
        goto(`/projects/${projects[0].id}/services/new`);
      }
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>Select Project for Service | Klouds</title>
</svelte:head>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Services' },
    { label: 'Select Project' }
  ]}
  backHref="/"
/>

<div class="select-project-container">
  <div class="page-header mb-4">
    <div class="flex items-center gap-3">
      <div class="header-icon">
        <Server size={22} />
      </div>
      <div>
        <h1 class="page-title">Deploy New Service</h1>
        <p class="page-subtitle">Select a project namespace to deploy your microservice into</p>
      </div>
    </div>
  </div>

  {#if loading}
    <div class="card p-5 text-center text-muted">Loading available projects...</div>
  {:else if projects.length === 0}
    <div class="card p-5 text-center">
      <FolderKanban size={36} class="text-muted mb-2" />
      <h3>No Projects Found</h3>
      <p class="text-sm text-muted mb-4">You need to create a project before deploying services.</p>
      <a href="/projects/new" class="btn btn-primary">
        <Plus size={14} />
        <span>Create Project</span>
      </a>
    </div>
  {:else}
    <div class="projects-list">
      {#each projects as proj}
        <a href={`/projects/${proj.id}/services/new`} class="project-item-card card">
          <div class="flex items-center gap-3">
            <div class="project-icon">
              <FolderKanban size={18} />
            </div>
            <div>
              <h3 class="text-sm font-bold text-white mb-1">{proj.name}</h3>
              <p class="text-xs text-muted mb-0">{proj.description || 'No description'}</p>
            </div>
          </div>
          <div class="flex items-center gap-2 text-xs text-accent">
            <span>Select Project</span>
            <ArrowRight size={14} />
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>

<style>
  .select-project-container {
    max-width: 680px;
    margin: 0 auto;
    padding-bottom: 3rem;
  }

  .header-icon {
    width: 44px;
    height: 44px;
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent);
  }

  .projects-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .project-item-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.25rem;
    text-decoration: none;
    transition: all 0.15s ease;
  }

  .project-item-card:hover {
    border-color: rgba(255, 255, 255, 0.25);
    background: rgba(255, 255, 255, 0.04);
  }

  .project-icon {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.06);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink);
  }
</style>
