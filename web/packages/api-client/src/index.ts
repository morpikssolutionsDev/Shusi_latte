import createClient from 'openapi-fetch';
import type { components, paths } from './schema';

export type Schemas = components['schemas'];
export type Principal = Schemas['Principal'];
export type Problem = Schemas['Problem'];
export type DemoAccount = Schemas['DemoAccount'];
export type OverrideAction = Schemas['OverrideAction'];

/** Typed REST client for /api/v1 (cookie session, same origin through Vite or Caddy). */
export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'include' });

/** A new Idempotency-Key for one user action; reuse it when retrying that action (TR-API-01). */
export function idempotencyKey(): string {
  return crypto.randomUUID();
}

/** Thrown for RFC 9457 problem responses, carrying the stable `code` (TR-API-02). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly detail?: string;
  readonly extra: Record<string, unknown>;

  constructor(problem: Problem) {
    super(problem.title);
    this.status = problem.status;
    this.code = problem.code;
    this.detail = problem.detail;
    this.extra = problem.extra ?? {};
  }
}

/** Unwraps an openapi-fetch result, throwing ApiError on a problem response. */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error !== undefined || !result.response.ok) {
    const p = result.error as Partial<Problem> | undefined;
    throw new ApiError({
      status: p?.status ?? result.response.status,
      code: p?.code ?? 'network',
      title: p?.title ?? result.response.statusText,
      detail: p?.detail,
      extra: p?.extra,
    });
  }
  return result.data as T;
}

export { connectRealtime, type RealtimeEvent, type RealtimeConnection } from './realtime';
