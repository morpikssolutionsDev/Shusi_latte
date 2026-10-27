import { api, idempotencyKey, unwrap, type Schemas } from '@sushi-latte/api-client';
import { queryOptions, useQueryClient } from '@tanstack/react-query';
import { useRealtime } from '../realtime';

export type AttendanceRow = Schemas['AttendanceRow'];
export type ShiftView = Schemas['ShiftView'];
export type LeaveView = Schemas['LeaveView'];
export type Employee = Schemas['Employee'];
export type ClockResult = Schemas['ClockResult'];
export type OpenClaim = Schemas['OpenClaim'];

/** Calendar dates in WIB, the outlet's zone. */
export function wibDate(ms = Date.now()) {
  return new Date(ms + 7 * 3600_000).toISOString().slice(0, 10);
}
export function addDays(date: string, n: number) {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}
/** Milliseconds for a WIB date and HH:MM. */
export function wibMs(date: string, time: string) {
  return Date.parse(`${date}T${time}:00+07:00`);
}
export function wibTime(ms: number) {
  return new Date(ms + 7 * 3600_000).toISOString().slice(11, 16);
}

export const meQuery = queryOptions({ queryKey: ['hris', 'me'], queryFn: async () => unwrap(await api.GET('/hris/me')) });
export const leaveQuery = queryOptions({ queryKey: ['hris', 'leave'], queryFn: async () => unwrap(await api.GET('/leave-requests')) });
export const employeesQuery = queryOptions({ queryKey: ['hris', 'employees'], queryFn: async () => unwrap(await api.GET('/employees')) });
export const attendanceQuery = (from: string, to: string) =>
  queryOptions({
    queryKey: ['hris', 'attendance', from, to],
    queryFn: async () => unwrap(await api.GET('/attendance', { params: { query: { from, to } } })),
    refetchInterval: 60_000,
  });
export const scheduleQuery = (from: string, to: string) =>
  queryOptions({ queryKey: ['hris', 'schedule', from, to], queryFn: async () => unwrap(await api.GET('/shifts', { params: { query: { from, to } } })) });

const idem = () => ({ 'Idempotency-Key': idempotencyKey() });

export const hris = {
  clock: async (body: Schemas['ClockRequest']) => unwrap(await api.POST('/attendance/clock', { body, headers: idem() })),
  correct: async (id: string, body: Schemas['CorrectAttendanceRequest']) =>
    unwrap(await api.POST('/attendance/{id}/correct', { params: { path: { id } }, body, headers: idem() })),
  createShift: async (body: Schemas['ShiftRequest']) => unwrap(await api.POST('/shifts', { body, headers: idem() })),
  updateShift: async (id: string, body: Schemas['ShiftRequest']) => unwrap(await api.PUT('/shifts/{id}', { params: { path: { id } }, body, headers: idem() })),
  deleteShift: async (id: string) => unwrap(await api.DELETE('/shifts/{id}', { params: { path: { id } }, headers: idem() })),
  requestLeave: async (body: Schemas['LeaveRequest']) => unwrap(await api.POST('/leave-requests', { body, headers: idem() })),
  decideLeave: async (id: string, approve: boolean, reason = '') =>
    unwrap(await api.POST('/leave-requests/{id}/decision', { params: { path: { id } }, body: { approve, reason }, headers: idem() })),
  saveEmployee: async (id: string, body: Schemas['EmployeeRequest']) =>
    unwrap(await api.PUT('/employees/{id}', { params: { path: { id } }, body, headers: idem() })),
};

/** Attendance, shift and leave changes refresh every HRIS view. */
export function useHrisRealtime() {
  const qc = useQueryClient();
  return useRealtime(['hris', 'cash'], () => void qc.invalidateQueries({ queryKey: ['hris'] }));
}
