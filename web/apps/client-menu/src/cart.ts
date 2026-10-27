// The Customer's cart before "Kirim pesanan". It holds choices only; prices, discounts and
// Dietary Warnings come from the server quote (CLAUDE.md: totals are decided in Go).
import { useSyncExternalStore } from 'react';
import type { CartItem } from './api';

export interface CartLine {
  key: string;
  productId: string;
  variantId: string;
  modifierIds: string[];
  quantity: number;
  note: string;
  acknowledgedTagIds: string[];
}

let owner = '';
let lines: CartLine[] = [];
const listeners = new Set<() => void>();
const storageKey = (customerId: string) => `sl.cart.${customerId}`;

function persist() {
  try {
    if (owner) localStorage.setItem(storageKey(owner), JSON.stringify(lines));
  } catch {
    // cart lives in memory only
  }
  listeners.forEach((l) => l());
}

/** Switches the cart to a Customer, loading what they left in it. */
export function loadCart(customerId: string) {
  if (owner === customerId) return;
  owner = customerId;
  try {
    lines = JSON.parse(localStorage.getItem(storageKey(customerId)) ?? '[]') as CartLine[];
  } catch {
    lines = [];
  }
  listeners.forEach((l) => l());
}

const sameChoice = (a: Omit<CartLine, 'key' | 'quantity'>, b: CartLine) =>
  a.variantId === b.variantId && a.note === b.note && [...a.modifierIds].sort().join() === [...b.modifierIds].sort().join();

export const cart = {
  add(line: Omit<CartLine, 'key'>) {
    const same = lines.find((l) => sameChoice(line, l));
    lines = same
      ? lines.map((l) => (l === same ? { ...l, quantity: Math.min(20, l.quantity + line.quantity) } : l))
      : [...lines, { ...line, key: crypto.randomUUID() }];
    persist();
  },
  setQuantity(key: string, quantity: number) {
    lines = lines.map((l) => (l.key === key ? { ...l, quantity: Math.max(1, Math.min(20, quantity)) } : l));
    persist();
  },
  remove(key: string): CartLine | undefined {
    const removed = lines.find((l) => l.key === key);
    lines = lines.filter((l) => l.key !== key);
    persist();
    return removed;
  },
  restore(line: CartLine, index: number) {
    lines = [...lines.slice(0, index), line, ...lines.slice(index)];
    persist();
  },
  acknowledge(key: string, tagIds: string[]) {
    lines = lines.map((l) => (l.key === key ? { ...l, acknowledgedTagIds: [...new Set([...l.acknowledgedTagIds, ...tagIds])] } : l));
    persist();
  },
  clear() {
    lines = [];
    persist();
  },
};

export const toItems = (ls: CartLine[]): CartItem[] =>
  ls.map((l) => ({ variantId: l.variantId, modifierIds: l.modifierIds, quantity: l.quantity, note: l.note, acknowledgedTagIds: l.acknowledgedTagIds }));

export function useCart(): CartLine[] {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => lines,
  );
}
