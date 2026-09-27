<script lang="ts">
  import { toggleMobileNav, isMobileNavOpen } from '$lib/stores/ui';
  import { Menu, X } from '@lucide/svelte';
</script>

<header class="mobile-top-header">
  <a href="/" class="mobile-logo-link">
    <div class="sidebar-logo-mark">K</div>
    <span class="mobile-brand-title">Klouds</span>
  </a>

  <button class="mobile-menu-btn" onclick={toggleMobileNav} aria-label="Toggle navigation menu">
    {#if $isMobileNavOpen}
      <X size={20} />
    {:else}
      <Menu size={20} />
    {/if}
  </button>
</header>

<style>
  .mobile-logo-link {
    display: flex;
    align-items: center;
    gap: 10px;
    text-decoration: none;
  }

  .mobile-brand-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--color-ink);
  }

  .mobile-menu-btn {
    background: transparent;
    border: none;
    color: var(--color-ink);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 6px;
  }
</style>
