import { api, ApiError, unwrap, type Principal } from '@sushi-latte/api-client';
import { queryOptions } from '@tanstack/react-query';

/** The signed-in principal, or null when signed out. */
export const meQuery = queryOptions({
  queryKey: ['me'],
  queryFn: async (): Promise<Principal | null> => {
    try {
      return unwrap(await api.GET('/auth/me'));
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) return null;
      throw e;
    }
  },
  staleTime: 60_000,
});

export const healthQuery = queryOptions({
  queryKey: ['health'],
  queryFn: async (): Promise<{ ok: boolean; demo: boolean }> => {
    const res = await fetch('/healthz');
    return res.json();
  },
  staleTime: Infinity,
});
