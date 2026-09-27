// UI state store

import { writable } from 'svelte/store';

export const isMobileNavOpen = writable<boolean>(false);

export function toggleMobileNav(): void {
  isMobileNavOpen.update(v => !v);
}

export function closeMobileNav(): void {
  isMobileNavOpen.set(false);
}
