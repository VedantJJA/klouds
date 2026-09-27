<script lang="ts">
  import { auth } from '$lib/stores/auth';
  import { Lock, Mail, AlertCircle, ArrowRight } from '@lucide/svelte';

  let email = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!email || !password) {
      error = 'Please provide both email and password';
      return;
    }

    loading = true;
    error = '';

    try {
      await auth.login({ email, password });
    } catch (err: any) {
      error = err.message || 'Login failed. Please check your credentials.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="auth-card">
  <div class="auth-header">
    <div class="sidebar-logo-mark auth-logo">K</div>
    <h2>Sign in to Klouds</h2>
    <p class="auth-subtitle">Enter your credentials to access your control panel</p>
  </div>

  {#if error}
    <div class="error-banner">
      <AlertCircle size={16} />
      <span>{error}</span>
    </div>
  {/if}

  <form onsubmit={handleSubmit} class="auth-form">
    <div class="form-group">
      <label class="form-label" for="login-email">Email Address</label>
      <div class="input-with-icon">
        <Mail size={16} class="input-icon" />
        <input
          id="login-email"
          type="email"
          class="form-input has-icon"
          placeholder="user@domain.internal"
          bind:value={email}
          required
          autocomplete="email"
        />
      </div>
    </div>

    <div class="form-group">
      <label class="form-label" for="login-password">Password</label>
      <div class="input-with-icon">
        <Lock size={16} class="input-icon" />
        <input
          id="login-password"
          type="password"
          class="form-input has-icon"
          placeholder="Password"
          bind:value={password}
          required
          autocomplete="current-password"
        />
      </div>
    </div>

    <button type="submit" class="btn btn-primary w-full" disabled={loading}>
      {#if loading}
        <span>Authenticating...</span>
      {:else}
        <span>Sign In</span>
        <ArrowRight size={16} />
      {/if}
    </button>
  </form>

  <div class="auth-footer">
    <span class="text-muted">Do not have an account?</span>
    <a href="/register" class="auth-link">Create an account</a>
  </div>
</div>

<style>
  .auth-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 400px;
    padding: 2.25rem 2rem;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.45);
  }

  .auth-header {
    text-align: center;
    margin-bottom: var(--sp-5);
  }

  .auth-logo {
    margin: 0 auto var(--sp-3);
    width: 36px;
    height: 36px;
    font-size: 1.125rem;
  }

  .auth-subtitle {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
    margin-top: 4px;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    background: var(--color-danger-subtle);
    border: 1px solid rgba(248, 113, 113, 0.3);
    border-radius: var(--radius-md);
    color: var(--color-danger);
    font-size: 0.8125rem;
    margin-bottom: var(--sp-4);
  }

  .auth-form {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }

  .input-with-icon {
    position: relative;
    display: flex;
    align-items: center;
  }

  :global(.input-icon) {
    position: absolute;
    left: 12px;
    color: var(--color-ink-muted);
    pointer-events: none;
  }

  .form-input.has-icon {
    padding-left: 38px;
  }

  .auth-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    margin-top: var(--sp-5);
    padding-top: var(--sp-4);
    border-top: 1px solid var(--color-border-subtle);
    font-size: 0.8125rem;
  }

  .auth-link {
    color: var(--color-ink);
    font-weight: 600;
  }

  .auth-link:hover {
    text-decoration: underline;
  }
</style>
