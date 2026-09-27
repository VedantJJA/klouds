<script lang="ts">
  import { page } from '$app/stores';
  import { currentUser, isAdmin, auth } from '$lib/stores/auth';
  import { theme } from '$lib/stores/theme';
  import { isMobileNavOpen, closeMobileNav } from '$lib/stores/ui';
  import {
    LayoutDashboard,
    FolderKanban,
    Database,
    Users,
    Activity,
    Server,
    LogOut,
    Sun,
    Moon,
    X
  } from '@lucide/svelte';

  const pathname = $derived($page.url.pathname);

  function isLinkActive(path: string, exact = false): boolean {
    if (exact) {
      return pathname === path;
    }
    return pathname.startsWith(path);
  }
</script>

<aside class="sidebar" class:open={$isMobileNavOpen}>
  <div class="sidebar-logo">
    <div class="sidebar-logo-mark">K</div>
    <span class="sidebar-logo-text">Klouds</span>
    <button class="mobile-close-btn" onclick={closeMobileNav} aria-label="Close navigation menu">
      <X size={18} />
    </button>
  </div>

  <nav class="sidebar-nav">
    <div class="sidebar-section-label">Platform</div>
    <a
      href="/"
      class="nav-item"
      class:active={isLinkActive('/', true)}
      onclick={closeMobileNav}
    >
      <span class="nav-item-icon">
        <LayoutDashboard size={18} />
      </span>
      <span>Overview</span>
    </a>

    <a
      href="/projects"
      class="nav-item"
      class:active={isLinkActive('/projects') || isLinkActive('/services')}
      onclick={closeMobileNav}
    >
      <span class="nav-item-icon">
        <FolderKanban size={18} />
      </span>
      <span>Projects</span>
    </a>

    <a
      href="/databases"
      class="nav-item"
      class:active={isLinkActive('/databases')}
      onclick={closeMobileNav}
    >
      <span class="nav-item-icon">
        <Database size={18} />
      </span>
      <span>Databases</span>
    </a>

    {#if $isAdmin}
      <div class="sidebar-section-label">Admin Console</div>
      <a
        href="/admin/users"
        class="nav-item"
        class:active={isLinkActive('/admin/users')}
        onclick={closeMobileNav}
      >
        <span class="nav-item-icon">
          <Users size={18} />
        </span>
        <span>Users & Quotas</span>
      </a>

      <a
        href="/admin/services"
        class="nav-item"
        class:active={isLinkActive('/admin/services')}
        onclick={closeMobileNav}
      >
        <span class="nav-item-icon">
          <Server size={18} />
        </span>
        <span>All Services</span>
      </a>

      <a
        href="/admin/metrics"
        class="nav-item"
        class:active={isLinkActive('/admin/metrics')}
        onclick={closeMobileNav}
      >
        <span class="nav-item-icon">
          <Activity size={18} />
        </span>
        <span>VM Telemetry</span>
      </a>
    {/if}
  </nav>

  <div class="sidebar-footer">
    {#if $currentUser}
      <div class="user-info-row">
        <div class="user-meta">
          <span class="user-name">{$currentUser.username}</span>
          <span class="badge badge-running">{$currentUser.role}</span>
        </div>
      </div>
    {/if}

    <div class="footer-actions">
      <button class="nav-item" onclick={() => theme.toggle()} aria-label="Toggle theme">
        <span class="nav-item-icon">
          {#if $theme === 'dark'}
            <Sun size={16} />
          {:else}
            <Moon size={16} />
          {/if}
        </span>
        <span>{$theme === 'dark' ? 'Light Mode' : 'Dark Mode'}</span>
      </button>

      <button class="nav-item text-danger" onclick={() => auth.logout()} aria-label="Sign out">
        <span class="nav-item-icon">
          <LogOut size={16} />
        </span>
        <span>Sign Out</span>
      </button>
    </div>
  </div>
</aside>

<style>
  .mobile-close-btn {
    display: none;
    background: none;
    border: none;
    color: var(--color-ink-muted);
    cursor: pointer;
    margin-left: auto;
    padding: 4px;
  }

  .user-info-row {
    padding: 6px 10px;
    background: var(--color-surface-subtle);
    border-radius: var(--radius-md);
    margin-bottom: 6px;
  }

  .user-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .user-name {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .footer-actions {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  @media (max-width: 960px) {
    .mobile-close-btn {
      display: flex;
      align-items: center;
      justify-content: center;
    }
  }
</style>
