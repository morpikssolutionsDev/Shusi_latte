import { api, unwrap } from '@sushi-latte/api-client';
import { Button, Card, Input, formatDateTime } from '@sushi-latte/ui';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { crewQuery } from './api';
import { PageHeader } from './layout';

const PAGE = 50;

/** Audit log viewer (BR-ADM-12): newest first, filter by action prefix, paged by time. */
export function AuditPage({ canListCrew }: { canListCrew: boolean }) {
  const { t } = useTranslation();
  const [action, setAction] = useState('');
  const crew = useQuery({ ...crewQuery, enabled: canListCrew });
  const name = (id?: string) => (id ? crew.data?.find((c) => c.id === id)?.fullName ?? id.slice(0, 8) : '');
  const entries = useInfiniteQuery({
    queryKey: ['audit', action],
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) =>
      unwrap(await api.GET('/audit', { params: { query: { action: action || undefined, before: pageParam, limit: PAGE } } })),
    getNextPageParam: (last) => (last.length === PAGE ? last[last.length - 1]!.at : undefined),
  });
  const rows = entries.data?.pages.flat() ?? [];

  return (
    <>
      <PageHeader title={t('admin.audit.title')} />
      <Input aria-label={t('admin.audit.filterAction')} placeholder={t('admin.audit.filterAction')} value={action}
        onChange={(e) => setAction(e.target.value.trim())} className="mb-4 max-w-sm font-mono" />
      <Card className="overflow-x-auto p-0">
        <table className="w-full min-w-[720px] text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-muted-foreground">
            <tr className="border-b border-border-subtle">
              <th className="px-4 py-3">{t('admin.audit.when')}</th>
              <th className="px-4 py-3">{t('admin.audit.action')}</th>
              <th className="px-4 py-3">{t('admin.audit.who')}</th>
              <th className="px-4 py-3">{t('admin.audit.approver')}</th>
              <th className="px-4 py-3">{t('admin.audit.detail')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((e) => (
              <tr key={e.id} className="border-b border-border-subtle align-top last:border-0">
                <td className="tabular whitespace-nowrap px-4 py-2.5">{formatDateTime(e.at)}</td>
                <td className="px-4 py-2.5 font-mono text-xs">{e.action}</td>
                <td className="px-4 py-2.5">{name(e.actorId)}</td>
                <td className="px-4 py-2.5">{name(e.approverId)}</td>
                <td className="max-w-md px-4 py-2.5 text-xs text-muted-foreground">
                  {e.reason && <span className="block text-foreground">{e.reason}</span>}
                  {e.before !== undefined && <span className="block truncate font-mono">− {JSON.stringify(e.before)}</span>}
                  {e.after !== undefined && <span className="block truncate font-mono">+ {JSON.stringify(e.after)}</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {entries.hasNextPage && (
        <Button variant="secondary" className="mt-4" disabled={entries.isFetchingNextPage} onClick={() => void entries.fetchNextPage()}>
          {t('admin.audit.loadMore')}
        </Button>
      )}
    </>
  );
}

