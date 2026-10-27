import { ApiError, api, connectRealtime, unwrap, type Schemas } from '@sushi-latte/api-client';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { getSession, setEnded, type GuestSession, type Ended } from './session';

export type Menu = Schemas['Menu'];
export type Product = Schemas['Product'];
export type Variant = Schemas['Variant'];
export type DietaryTag = Schemas['DietaryTag'];
export type CustomerView = Schemas['CustomerView'];
export type OrderView = Schemas['OrderView'];
export type ItemView = Schemas['ItemView'];
export type CartItem = Schemas['CartItem'];
export type Quote = Schemas['Quote'];
export type ItemProblem = Schemas['ItemProblem'];

const GUEST_HEADER = 'X-Guest-Token';
const ENDING: Ended[] = ['continued_elsewhere', 'bill_closed', 'guest_invalid'];

// Every request carries this tab's guest token; a 401 that ends the session switches the app
// to the matching notice instead of failing silently.
api.use({
  onRequest({ request }) {
    const s = getSession();
    if (s && !request.headers.has(GUEST_HEADER)) request.headers.set(GUEST_HEADER, s.token);
    return request;
  },
  async onResponse({ request, response }) {
    if (response.status === 401 && request.headers.has(GUEST_HEADER)) {
      const body = (await response.clone().json().catch(() => ({}))) as { code?: string };
      if (ENDING.includes(body.code as Ended)) setEnded(body.code as Ended);
    }
    return response;
  },
});

export const healthQuery = queryOptions({
  queryKey: ['health'],
  queryFn: async (): Promise<{ ok: boolean; demo: boolean }> => (await fetch('/healthz')).json(),
  staleTime: Infinity,
});

export const menuQuery = queryOptions({
  queryKey: ['menu'],
  queryFn: async () => unwrap(await api.GET('/public/catalog')),
  staleTime: 30_000,
});

export const tableQuery = (qr: string) =>
  queryOptions({
    queryKey: ['table', qr],
    queryFn: async () => unwrap(await api.GET('/public/tables/{qr}', { params: { path: { qr } } })),
    staleTime: 0,
  });

export const meQuery = (customerId: string) =>
  queryOptions({
    queryKey: ['me', customerId],
    queryFn: async () => unwrap(await api.GET('/public/bills/me')),
    // Pending Orders change only through events; this catches anything missed.
    refetchInterval: 30_000,
  });

export const problemOf = (e: unknown) => (e instanceof ApiError ? e : null);

/**
 * Follows the menu, this Customer's Bill and their own customer topic (TR-RT-02). Events carry
 * ids only, so everything is a refetch (TR-RT-03). Returns whether the socket is connected.
 */
export function useGuestRealtime(session: GuestSession | null, billId: string | undefined) {
  const qc = useQueryClient();
  const [online, setOnline] = useState(true);
  useEffect(() => {
    if (!session) return;
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const bill = billId ?? session.billId;
    const conn = connectRealtime({
      url: `${proto}://${location.host}/ws?guest=${encodeURIComponent(session.token)}`,
      topics: ['menu', `bill:${bill}`, `customer:${session.customerId}`],
      onEvent: (e) => {
        if (e.topic === 'menu') void qc.invalidateQueries({ queryKey: ['menu'] });
        else if (e.type === 'customer.revoked') setEnded('continued_elsewhere');
        else if (e.type === 'bill.closed') setEnded('bill_closed');
        else void qc.invalidateQueries({ queryKey: ['me'] });
      },
      onReconnect: () => void qc.invalidateQueries(),
      onStatus: (s) => setOnline(s === 'open'),
    });
    return () => conn.close();
  }, [session, billId, qc]);
  return online;
}
