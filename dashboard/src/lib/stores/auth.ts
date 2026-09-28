// Authentication and session store

import { writable, derived } from 'svelte/store';
import { api, getAuthToken, setAuthToken, clearAuthToken, type User, type Quota } from '$lib/api/client';
import { goto } from '$app/navigation';

interface AuthState {
  user: User | null;
  quota: Quota | null;
  token: string | null;
  loading: boolean;
  initialized: boolean;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    quota: null,
    token: null,
    loading: true,
    initialized: false
  });

  return {
    subscribe,
    init: async () => {
      const token = getAuthToken();
      if (!token) {
        update(s => ({ ...s, token: null, user: null, loading: false, initialized: true }));
        return;
      }

      update(s => ({ ...s, token, loading: true }));
      try {
        const res = await api.me();
        update(s => ({
          ...s,
          user: res.user,
          quota: res.quota || null,
          loading: false,
          initialized: true
        }));
      } catch {
        clearAuthToken();
        update(s => ({ ...s, user: null, quota: null, token: null, loading: false, initialized: true }));
      }
    },
    login: async (creds: { email: string; password: string }) => {
      const res = await api.login(creds);
      setAuthToken(res.token);
      update(s => ({
        ...s,
        token: res.token,
        user: res.user,
        loading: false,
        initialized: true
      }));

      if (res.user.status === 'pending') {
        goto('/access/pending');
      } else {
        goto('/');
      }
      return res;
    },
    loginWithToken: async (token: string, redirectUrl?: string) => {
      setAuthToken(token);
      update(s => ({ ...s, token, loading: true }));
      try {
        const res = await api.me();
        update(s => ({
          ...s,
          user: res.user,
          quota: res.quota || null,
          loading: false,
          initialized: true
        }));
        if (res.user.status === 'pending') {
          goto('/access/pending');
        } else if (redirectUrl && !redirectUrl.includes('://')) {
          goto(redirectUrl);
        } else {
          goto('/');
        }
      } catch {
        clearAuthToken();
        update(s => ({ ...s, user: null, quota: null, token: null, loading: false, initialized: true }));
      }
    },
    register: async (data: { email: string; username: string; password: string }) => {
      const res = await api.register(data);
      setAuthToken(res.token);
      update(s => ({
        ...s,
        token: res.token,
        user: res.user,
        loading: false,
        initialized: true
      }));

      if (res.user.status === 'pending') {
        goto('/access/pending');
      } else {
        goto('/');
      }
      return res;
    },
    logout: () => {
      clearAuthToken();
      update(s => ({
        ...s,
        user: null,
        quota: null,
        token: null,
        loading: false,
        initialized: true
      }));
      goto('/login');
    },
    refreshUser: async () => {
      try {
        const res = await api.me();
        update(s => ({ ...s, user: res.user, quota: res.quota || null }));
      } catch {
        // Token might have expired
      }
    }
  };
}

export const auth = createAuthStore();

export const isAuthenticated = derived(auth, $auth => !!$auth.user && !!$auth.token);
export const isAdmin = derived(auth, $auth => $auth.user?.role === 'admin');
export const isPending = derived(auth, $auth => $auth.user?.status === 'pending');
export const currentUser = derived(auth, $auth => $auth.user);
export const userQuota = derived(auth, $auth => $auth.quota);
