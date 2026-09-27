<script lang="ts">
  import { ChevronRight, ArrowLeft } from '@lucide/svelte';

  interface BreadcrumbItem {
    label: string;
    href?: string;
  }

  let {
    items = [] as BreadcrumbItem[],
    backHref = '' as string
  } = $props();

  // If backHref is not provided, use the second to last item with an href
  const effectiveBackHref = $derived.by(() => {
    if (backHref) return backHref;
    if (items.length >= 2) {
      return items[items.length - 2]?.href || '/';
    }
    return '';
  });
</script>

<nav class="breadcrumb-container" aria-label="Breadcrumb">
  {#if effectiveBackHref}
    <a href={effectiveBackHref} class="btn-back" title="Go to previous page">
      <ArrowLeft size={14} />
      <span>Back</span>
    </a>
  {/if}

  <div class="breadcrumb-list">
    {#each items as item, index}
      {#if index > 0}
        <span class="breadcrumb-separator" aria-hidden="true">
          <ChevronRight size={13} />
        </span>
      {/if}

      {#if item.href && index < items.length - 1}
        <a href={item.href} class="breadcrumb-link">{item.label}</a>
      {:else}
        <span class="breadcrumb-current" aria-current="page">{item.label}</span>
      {/if}
    {/each}
  </div>
</nav>

<style>
  .breadcrumb-container {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: var(--sp-3);
    font-size: 0.8125rem;
  }

  .btn-back {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-ink-secondary);
    font-size: 0.75rem;
    font-weight: 500;
    text-decoration: none;
    transition: all var(--transition-fast);
  }

  .btn-back:hover {
    color: var(--color-ink);
    border-color: var(--color-border-focus);
    background: var(--color-surface);
  }

  .breadcrumb-list {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .breadcrumb-separator {
    color: var(--color-ink-muted);
    display: flex;
    align-items: center;
  }

  .breadcrumb-link {
    color: var(--color-ink-secondary);
    text-decoration: none;
    transition: color var(--transition-fast);
  }

  .breadcrumb-link:hover {
    color: var(--color-ink);
  }

  .breadcrumb-current {
    color: var(--color-ink);
    font-weight: 500;
  }
</style>
