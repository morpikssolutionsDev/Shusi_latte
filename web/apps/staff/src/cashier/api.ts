import { api, idempotencyKey, unwrap, type Schemas } from '@sushi-latte/api-client';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import { useRealtime } from '../realtime';

export type Drawer = Schemas['Drawer'];
export type BillBrief = Schemas['BillBrief'];
export type HeldCash = Schemas['HeldCash'];
export type PendingQRIS = Schemas['PendingQRIS'];
export type OutageRequest = Schemas['OutageRequest'];

// Keys live under the shared floor-staff prefix ['waiter'], so the Bill actions reused from the
// Waiter module (which invalidate ['waiter']) refresh the Cashier views too.
export const cashierBillsQuery = queryOptions({
  queryKey: ['waiter', 'cashier', 'bills'],
  queryFn: async () => unwrap(await api.GET('/cashier/bills')),
  refetchInterval: 30_000,
});

export const drawerQuery = queryOptions({
  queryKey: ['waiter', 'cashier', 'drawer'],
  queryFn: async () => unwrap(await api.GET('/drawer')),
  refetchInterval: 30_000,
});

const idem = () => ({ 'Idempotency-Key': idempotencyKey() });

export const cashier = {
  openDrawer: async (openingFloat: number) => unwrap(await api.POST('/drawer-sessions', { body: { openingFloat }, headers: idem() })),
  closeDrawer: async (countedCash: number) => unwrap(await api.POST('/drawer-sessions/current/close', { body: { countedCash }, headers: idem() })),
  handover: async (waiterId: string, receivedAmount: number) =>
    unwrap(await api.POST('/cash-handovers', { body: { waiterId, receivedAmount }, headers: idem() })),
  verify: async (id: string) => unwrap(await api.POST('/payments/{id}/verify', { params: { path: { id } }, headers: idem() })),
  writeOff: async (id: string, reason: string) =>
    unwrap(await api.POST('/payments/{id}/write-off', { params: { path: { id } }, body: { reason }, headers: idem() })),
  refund: async (id: string, amount: number, reason: string, token: string) =>
    unwrap(await api.POST('/bills/{id}/refunds', { params: { path: { id } }, body: { amount, reason }, headers: { ...idem(), 'X-Override-Token': token } })),
  takeaway: async (body: Schemas['TakeawayRequest']) => unwrap(await api.POST('/bills', { body, headers: idem() })),
  outage: async (body: OutageRequest) => unwrap(await api.POST('/post-outage/bills', { body, headers: idem() })),
};

/** Cashier realtime: Bill, floor and drawer events all refetch the Cashier views. */
export function useCashierRealtime() {
  const qc = useQueryClient();
  return useRealtime(['waiters', 'bill:*', 'cash', 'menu'], (e) => {
    if (e.topic === 'menu') void qc.invalidateQueries({ queryKey: ['catalog'] });
    void qc.invalidateQueries({ queryKey: ['waiter'] });
  });
}
