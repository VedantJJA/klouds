<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Service } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import { Server, RefreshCw, ExternalLink } from '@lucide/svelte';

  let services = $state<Service[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function loadServices() {
    loading = true;
    error = '';
    try {
      services = await api.listAllServices();
    } catch (err: any) {
      error = err.message || 'Failed to load services';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadServices();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Admin' },
    { label: 'All Services' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title">Global Services Inventory</h1>
    <p class="page-subtitle">Overview of all application containers running across every tenant and project</p>
  </div>

  <button class="btn btn-secondary btn-sm" onclick={loadServices} disabled={loading}>
    <RefreshCw size={14} class={loading ? 'spin' : ''} />
    <span>Refresh</span>
  </button>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

{#if loading}
  <div class="card text-center p-5 text-muted">Auditing active workloads...</div>
{:else if services.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon">
      <Server size={36} />
    </div>
    <h3>No Services Deployed</h3>
    <p class="text-sm text-muted">No active containers found in the cluster.</p>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Service Name</th>
          <th>Status</th>
          <th>Source</th>
          <th>Subdomain</th>
          <th>Internal Port</th>
          <th>RAM Limit</th>
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
            <td class="font-mono text-xs">{svc.memory_limit} MB</td>
            <td>
              <a href={`/services/${svc.id}`} class="btn btn-secondary btn-sm">
                Inspect
              </a>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
