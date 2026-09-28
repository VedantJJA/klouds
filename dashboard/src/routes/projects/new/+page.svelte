<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import { FolderKanban, Plus, ArrowLeft, ArrowRight, ShieldCheck } from '@lucide/svelte';

  let projectName = $state('');
  let projectDesc = $state('');
  let environment = $state<'production' | 'staging' | 'development'>('production');
  let creating = $state(false);
  let createError = $state('');

  let projectSlug = $derived(
    projectName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'my-project'
  );

  async function handleCreateProject(e: SubmitEvent) {
    e.preventDefault();
    if (!projectName.trim()) return;

    creating = true;
    createError = '';
    try {
      const created = await api.createProject({
        name: projectName.trim(),
        description: projectDesc.trim() || `Environment: ${environment}`
      });
      goto(`/projects/${created.id}`);
    } catch (err: any) {
      createError = err.message || 'Failed to create project';
      creating = false;
    }
  }
</script>

<svelte:head>
  <title>New Project | Klouds</title>
</svelte:head>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    { label: 'New Project' }
  ]}
  backHref="/projects"
/>

<div class="project-create-container">
  <div class="page-header mb-4">
    <div class="flex items-center gap-3">
      <div class="header-icon">
        <FolderKanban size={22} />
      </div>
      <div>
        <h1 class="page-title">New Project</h1>
        <p class="page-subtitle">Group and isolate your microservices, APIs, and managed databases</p>
      </div>
    </div>
  </div>

  <form onsubmit={handleCreateProject}>
    <div class="card p-5">
      {#if createError}
        <div class="error-banner mb-4">
          <span>{createError}</span>
        </div>
      {/if}

      <div class="form-grid">
        <div class="form-group">
          <label class="form-label" for="proj-name">
            Project Name <span style="color: var(--color-danger);">*</span>
          </label>
          <input
            id="proj-name"
            type="text"
            class="form-input"
            placeholder="e.g. ecommerce-platform or backend-cluster"
            bind:value={projectName}
            required
          />
          <span class="text-xs text-muted" style="margin-top: 4px;">
            Namespace slug: <code class="font-mono text-white">{projectSlug}</code>
          </span>
        </div>

        <div class="form-group">
          <span class="form-label">Target Environment</span>
          <div class="env-toggle-grid">
            <button
              type="button"
              class="env-card"
              class:selected={environment === 'production'}
              onclick={() => environment = 'production'}
            >
              <div class="env-badge prod">Production</div>
              <div class="env-sub">High availability, isolated VPC network, automated Let's Encrypt TLS</div>
            </button>

            <button
              type="button"
              class="env-card"
              class:selected={environment === 'staging'}
              onclick={() => environment = 'staging'}
            >
              <div class="env-badge staging">Staging</div>
              <div class="env-sub">Pre-production staging environment for integration testing</div>
            </button>

            <button
              type="button"
              class="env-card"
              class:selected={environment === 'development'}
              onclick={() => environment = 'development'}
            >
              <div class="env-badge dev">Development</div>
              <div class="env-sub">Rapid iteration sandbox with fast rebuilds</div>
            </button>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label" for="proj-desc">Description (Optional)</label>
          <textarea
            id="proj-desc"
            class="form-textarea"
            rows={3}
            placeholder="Brief description of the workloads, team, or purpose..."
            bind:value={projectDesc}
          ></textarea>
        </div>
      </div>
    </div>

    <div class="form-actions mt-4">
      <a href="/projects" class="btn btn-secondary">
        <ArrowLeft size={14} />
        <span>Cancel</span>
      </a>

      <button type="submit" class="btn btn-primary" disabled={creating || !projectName.trim()}>
        {#if creating}
          <span>Creating Project...</span>
        {:else}
          <Plus size={15} />
          <span>Create Project</span>
        {/if}
      </button>
    </div>
  </form>
</div>

<style>
  .project-create-container {
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

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .env-toggle-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.75rem;
  }

  .env-card {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.85rem;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .env-card:hover {
    border-color: rgba(255, 255, 255, 0.2);
    background: rgba(255, 255, 255, 0.04);
  }

  .env-card.selected {
    border-color: var(--color-accent);
    background: rgba(59, 130, 246, 0.08);
    box-shadow: 0 0 0 1px var(--color-accent);
  }

  .env-badge {
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    display: inline-block;
    padding: 2px 6px;
    border-radius: 4px;
    margin-bottom: 6px;
  }

  .env-badge.prod {
    background: rgba(34, 197, 94, 0.2);
    color: var(--color-success);
  }

  .env-badge.staging {
    background: rgba(234, 179, 8, 0.2);
    color: var(--color-warning);
  }

  .env-badge.dev {
    background: rgba(148, 163, 184, 0.2);
    color: var(--color-ink-secondary);
  }

  .env-sub {
    font-size: 0.72rem;
    color: var(--color-ink-secondary);
    line-height: 1.4;
  }

  .form-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
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
