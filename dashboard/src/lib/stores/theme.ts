// Theme store supporting dark and light themes with persistence

import { writable } from 'svelte/store';

export type ThemeMode = 'dark' | 'light';

function createThemeStore() {
  const { subscribe, set, update } = writable<ThemeMode>('dark');

  function applyTheme(mode: ThemeMode) {
    if (typeof document === 'undefined') return;
    document.documentElement.setAttribute('data-theme', mode);
    if (mode === 'dark') {
      document.documentElement.classList.add('dark');
      document.documentElement.classList.remove('light');
    } else {
      document.documentElement.classList.add('light');
      document.documentElement.classList.remove('dark');
    }
  }

  return {
    subscribe,
    init: () => {
      if (typeof window === 'undefined') return;
      const saved = localStorage.getItem('klouds_theme') as ThemeMode | null;
      const mode = saved === 'light' ? 'light' : 'dark';
      set(mode);
      applyTheme(mode);
    },
    toggle: () => {
      update(current => {
        const next: ThemeMode = current === 'dark' ? 'light' : 'dark';
        if (typeof window !== 'undefined') {
          localStorage.setItem('klouds_theme', next);
          applyTheme(next);
        }
        return next;
      });
    },
    setTheme: (mode: ThemeMode) => {
      if (typeof window !== 'undefined') {
        localStorage.setItem('klouds_theme', mode);
        applyTheme(mode);
      }
      set(mode);
    }
  };
}

export const theme = createThemeStore();
