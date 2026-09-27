<script lang="ts">
  import '../app.css';
  import Sidebar from '$lib/components/layout/Sidebar.svelte';
  import MobileHeader from '$lib/components/layout/MobileHeader.svelte';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { theme } from '$lib/stores/theme';
  import { auth, isAuthenticated, isPending } from '$lib/stores/auth';
  import { goto } from '$app/navigation';

  let { children } = $props();

  const pathname = $derived($page.url.pathname);
  const isAuthRoute = $derived(
    pathname.startsWith('/login') || 
    pathname.startsWith('/register') || 
    pathname.startsWith('/access/pending')
  );

  onMount(async () => {
    theme.init();
    await auth.init();

    // Check auth guards on mount
    const token = localStorage.getItem('klouds_auth_token');
    if (!token && !isAuthRoute) {
      goto('/login');
    }
  });

  // Reactive redirect guard for pending or unauthenticated users
  $effect(() => {
    if ($auth.initialized && !$auth.loading) {
      if (!$isAuthenticated && !isAuthRoute) {
        goto('/login');
      } else if ($isAuthenticated && $isPending && pathname !== '/access/pending') {
        goto('/access/pending');
      }
    }
  });
</script>

<svelte:head>
  <title>Klouds - Self-Hosted Cloud Platform</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta name="description" content="Self-hosted cloud platform for ARM64 with Dokploy and Render capabilities." />
</svelte:head>

{#if isAuthRoute}
  <div class="auth-wrapper">
    {@render children()}
  </div>
{:else}
  <div class="app-shell">
    <Sidebar />
    <div class="main-wrapper">
      <MobileHeader />
      <main class="main-content">
        {@render children()}
      </main>
    </div>
  </div>
{/if}

<style>
  .auth-wrapper {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-canvas);
    padding: 1.5rem;
  }
</style>
