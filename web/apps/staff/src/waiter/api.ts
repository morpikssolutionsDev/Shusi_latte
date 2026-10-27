import { api, idempotencyKey, unwrap, type Schemas } from '@sushi-latte/api-client';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import { useRef } from 'react';
import { useRealtime } from '../realtime';

export type Feed = Schemas['Feed'];
export type StaffOrder = Schemas['StaffOrder'];
export type StaffBill = Schemas['StaffBill'];
export type TableCard = Schemas['TableCard'];
export type CallView = Schemas['CallView'];
export type ItemView = Schemas['ItemView'];
export type CartItem = Schemas['CartItem'];
export type CancelReason = Schemas['CancelReason'];

export const feedQuery = queryOptions({
  queryKey: ['waiter', 'feed'],
  queryFn: async () => unwrap(await api.GET('/waiter/feed')),
  refetchInterval: 30_000,
});

export const floorQuery = queryOptions({
  queryKey: ['waiter', 'floor'],
  queryFn: async () => unwrap(await api.GET('/floor')),
  refetchInterval: 30_000,
});

export const billQuery = (id: string) =>
  queryOptions({
    queryKey: ['waiter', 'bill', id],
    queryFn: async () => unwrap(await api.GET('/bills/{id}', { params: { path: { id } } })),
  });

export const staffMenuQuery = queryOptions({
  queryKey: ['catalog', 'active'],
  queryFn: async () => unwrap(await api.GET('/catalog')),
});

// Every staff mutation carries an Idempotency-Key: tablet Wi-Fi retries must not double-act (TR-API-01).
const idem = () => ({ 'Idempotency-Key': idempotencyKey() });

export const actions = {
  claim: async (id: string) => unwrap(await api.POST('/orders/{id}/claim', { params: { path: { id } }, headers: idem() })),
  takeover: async (id: string) => unwrap(await api.POST('/orders/{id}/takeover', { params: { path: { id } }, headers: idem() })),
  confirm: async (id: string) => unwrap(await api.POST('/orders/{id}/confirm', { params: { path: { id } }, headers: idem() })),
  addItems: async (id: string, items: CartItem[]) =>
    unwrap(await api.POST('/orders/{id}/items', { params: { path: { id } }, body: { items }, headers: idem() })),
  direct: async (body: Schemas['DirectOrderRequest']) => unwrap(await api.POST('/orders', { body, headers: idem() })),
  cancel: async (id: string, reason: CancelReason, note: string) =>
    unwrap(await api.POST('/order-items/{id}/cancel', { params: { path: { id } }, body: { reason, note }, headers: idem() })),
  serve: async (itemIds: string[]) => unwrap(await api.POST('/order-items/serve', { body: { itemIds }, headers: idem() })),
  remake: async (id: string, reason: string, token: string) =>
    unwrap(await api.POST('/order-items/{id}/remake', { params: { path: { id } }, body: { reason }, headers: { ...idem(), 'X-Override-Token': token } })),
  void: async (id: string, reason: string, token: string) =>
    unwrap(await api.POST('/order-items/{id}/void', { params: { path: { id } }, body: { reason }, headers: { ...idem(), 'X-Override-Token': token } })),
  answer: async (id: string) => unwrap(await api.POST('/waiter-calls/{id}/answer', { params: { path: { id } }, headers: idem() })),

  // Bills and payment (OP-09, OP-10).
  settle: async (id: string) => unwrap(await api.POST('/bills/{id}/settle', { params: { path: { id } }, headers: idem() })),
  unlock: async (id: string) => unwrap(await api.POST('/bills/{id}/unlock', { params: { path: { id } }, headers: idem() })),
  pay: async (id: string, body: Schemas['PayRequest'], token?: string) =>
    unwrap(await api.POST('/bills/{id}/payments', { params: { path: { id } }, body, headers: { ...idem(), ...(token ? { 'X-Override-Token': token } : {}) } })),
  close: async (id: string, subBillId?: string, payNowServeLater = false) =>
    unwrap(await api.POST('/bills/{id}/close', { params: { path: { id } }, body: { subBillId, payNowServeLater }, headers: idem() })),
  unpaid: async (id: string, reason: string, token: string) =>
    unwrap(await api.POST('/bills/{id}/unpaid', { params: { path: { id } }, body: { reason }, headers: { ...idem(), 'X-Override-Token': token } })),
  discount: async (id: string, discountId: string, token: string) =>
    unwrap(await api.POST('/bills/{id}/discount', { params: { path: { id } }, body: { discountId }, headers: { ...idem(), 'X-Override-Token': token } })),
  move: async (id: string, toTableId: string, fromTableId?: string) =>
    unwrap(await api.POST('/bills/{id}/move', { params: { path: { id } }, body: { toTableId, fromTableId }, headers: idem() })),
  merge: async (intoBillId: string, billIds: string[]) => unwrap(await api.POST('/bills/merge', { body: { intoBillId, billIds }, headers: idem() })),
  split: async (id: string, body: Schemas['SplitRequest']) => unwrap(await api.POST('/bills/{id}/split', { params: { path: { id } }, body, headers: idem() })),
};

let audio: AudioContext | null = null;

/** Unlocks sound after a tap (browsers block audio until a user gesture). */
export function enableSound() {
  audio ??= new AudioContext();
  void audio.resume();
}
export const soundEnabled = () => audio?.state === 'running';

/** Two-tone chime + vibration for a new Claim notification (BR-WT-01, MOTION.md §7). */
export function attention() {
  navigator.vibrate?.([120, 60, 120]);
  if (!audio || audio.state !== 'running') return;
  const now = audio.currentTime;
  [880, 1175].forEach((freq, i) => {
    const osc = audio!.createOscillator();
    const gain = audio!.createGain();
    osc.frequency.value = freq;
    gain.gain.setValueAtTime(0.0001, now + i * 0.18);
    gain.gain.exponentialRampToValueAtTime(0.25, now + i * 0.18 + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + i * 0.18 + 0.3);
    osc.connect(gain).connect(audio!.destination);
    osc.start(now + i * 0.18);
    osc.stop(now + i * 0.18 + 0.32);
  });
}

const ALERTS = new Set(['order.pending', 'order.renotify', 'waiter_call.created', 'waiter_call.renotify', 'item.pickup_reminder', 'item.flagged']);

/** Waiter realtime: every event refetches the Waiter views; new work also chimes. */
export function useWaiterRealtime() {
  const qc = useQueryClient();
  const last = useRef(0);
  return useRealtime(['waiters', 'menu', 'bill:*'], (e) => {
    if (e.topic === 'menu') void qc.invalidateQueries({ queryKey: ['catalog'] });
    void qc.invalidateQueries({ queryKey: ['waiter'] });
    if (ALERTS.has(e.type) && Date.now() - last.current > 1500) {
      last.current = Date.now();
      attention();
    }
  });
}
