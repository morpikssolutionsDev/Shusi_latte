// Guest sessions (TR-AUTH-07). The token lives in sessionStorage, so every tab can be a
// different Customer (several Customers on one laptop in the demo), with a per-Table copy in
// localStorage so a phone that closed its tab resumes without re-entering its name.
import { useSyncExternalStore } from 'react';

export interface GuestSession {
  token: string;
  customerId: string;
  billId: string;
  name: string;
  qr: string;
}

const TAB_KEY = 'sl.guest';
const deviceKey = (qr: string) => `sl.guest.${qr}`;

function read(storage: Storage, key: string): GuestSession | null {
  try {
    const raw = storage.getItem(key);
    return raw ? (JSON.parse(raw) as GuestSession) : null;
  } catch {
    return null;
  }
}

function write(storage: Storage, key: string, value: GuestSession | null) {
  try {
    if (value) storage.setItem(key, JSON.stringify(value));
    else storage.removeItem(key);
  } catch {
    // storage blocked (private mode): the session lasts as long as the page
  }
}

let current: GuestSession | null = read(sessionStorage, TAB_KEY);
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

/** The tab's session for this Table, falling back to the device copy unless `fresh` (demo links). */
export function resume(qr: string, fresh: boolean): GuestSession | null {
  if (current?.qr === qr) return current;
  const device = fresh ? null : read(localStorage, deviceKey(qr));
  setSession(device);
  return device;
}

export function setSession(s: GuestSession | null) {
  current = s;
  write(sessionStorage, TAB_KEY, s);
  if (s) write(localStorage, deviceKey(s.qr), s);
  emit();
}

/** Forget the session everywhere (Bill closed, or continued on another device). */
export function clearSession(qr?: string) {
  if (qr) write(localStorage, deviceKey(qr), null);
  current = null;
  write(sessionStorage, TAB_KEY, null);
  emit();
}

export const getSession = () => current;

export function useSession(): GuestSession | null {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

/** Why a session ended, set from 401 problems and realtime events. */
export type Ended = 'continued_elsewhere' | 'bill_closed' | 'guest_invalid' | null;
let ended: Ended = null;
const endedListeners = new Set<() => void>();

export function setEnded(reason: Ended) {
  ended = reason;
  endedListeners.forEach((l) => l());
}

export function useEnded(): Ended {
  return useSyncExternalStore(
    (l) => {
      endedListeners.add(l);
      return () => endedListeners.delete(l);
    },
    () => ended,
  );
}

/** Forget this tab's and every per-Table session, e.g. after a demo reset made them all invalid. */
export function forgetAllSessions() {
  try {
    for (const key of Object.keys(localStorage)) if (key.startsWith('sl.guest.')) localStorage.removeItem(key);
  } catch {
    // storage blocked: nothing was kept
  }
  clearSession();
}
