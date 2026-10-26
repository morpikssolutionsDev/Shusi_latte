import { api, ApiError, unwrap, type Schemas } from '@sushi-latte/api-client';
import { queryOptions, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { toast } from 'sonner';
import i18n from '../i18n';

export type Catalog = Schemas['Catalog'];
export type Product = Schemas['Product'];
export type Variant = Schemas['Variant'];
export type ModifierGroup = Schemas['ModifierGroup'];
export type Category = Schemas['Category'];
export type DietaryTag = Schemas['DietaryTag'];
export type Discount = Schemas['Discount'];
export type Table = Schemas['Table'];
export type CrewMember = Schemas['CrewMember'];
export type CrewInput = Schemas['CrewInput'];
export type Settings = Schemas['Settings'];
export type AuditEntry = Schemas['AuditEntry'];

export const catalogQuery = queryOptions({
  queryKey: ['catalog', 'staff'],
  queryFn: async () => unwrap(await api.GET('/catalog', { params: { query: { archived: true } } })),
});
export const settingsQuery = queryOptions({
  queryKey: ['settings'],
  queryFn: async () => unwrap(await api.GET('/settings')),
});
export const discountsQuery = queryOptions({
  queryKey: ['discounts'],
  queryFn: async () => unwrap(await api.GET('/discounts')),
});
export const tablesQuery = queryOptions({
  queryKey: ['tables'],
  queryFn: async () => unwrap(await api.GET('/tables')),
});
export const crewQuery = queryOptions({
  queryKey: ['crew'],
  queryFn: async () => unwrap(await api.GET('/crew')),
});

export function errorText(e: unknown): string {
  if (e instanceof ApiError) return e.detail ?? e.message;
  return i18n.t('login.network');
}

/**
 * Mutation that refreshes the given queries and shows a toast. Problem `detail` texts from the
 * API are already Indonesian and user-facing (CONTENT.md §3), so they are shown as-is.
 */
export function useSave<TVars, TData>(
  fn: (vars: TVars) => Promise<TData>,
  invalidate: QueryKey[],
  successMessage?: string,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      invalidate.forEach((queryKey) => void queryClient.invalidateQueries({ queryKey }));
      if (successMessage) toast.success(successMessage);
    },
    onError: (e) => toast.error(errorText(e)),
  });
}

/** Bilingual name helper: staff screens read Indonesian. */
export const nameOf = (x: { nameId: string }) => x.nameId;

export function minutes(seconds: number) {
  return Math.round(seconds / 60);
}
