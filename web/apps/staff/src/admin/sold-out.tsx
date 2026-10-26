import { api, unwrap } from '@sushi-latte/api-client';
import { Card, SoldOutBadge, Switch, cn, formatTime, imageUrl } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { catalogQuery, useSave } from './api';
import { PageHeader } from './layout';

/** Sold Out per Variant for Outlet Admin and Management (BR-ADM-07). */
export function SoldOutPage() {
  const { t } = useTranslation();
  const { data } = useQuery(catalogQuery);
  const toggle = useSave(
    async ({ id, soldOut }: { id: string; soldOut: boolean }) =>
      unwrap(await api.PUT('/catalog/variants/{id}/sold-out', { params: { path: { id } }, body: { soldOut } })),
    [['catalog']],
  );
  if (!data) return null;
  const visible = data.products.filter((p) => p.status !== 'archived');

  return (
    <>
      <PageHeader title={t('admin.soldOut.title')}>{t('admin.soldOut.hint')}</PageHeader>
      {data.categories.map((c) => {
        const products = visible.filter((p) => p.categoryId === c.id);
        if (products.length === 0) return null;
        return (
          <section key={c.id} className="mb-8">
            <h2 className="mb-3 text-xs font-bold uppercase tracking-[0.12em] text-muted-foreground">{c.nameId}</h2>
            <div className="grid gap-3 md:grid-cols-2">
              {products.map((p) => (
                <Card key={p.id} className="flex gap-3 p-3">
                  {p.imageKey && (
                    <img src={imageUrl(p.imageKey, 'sq320')} alt="" className={cn('size-16 shrink-0 rounded-lg object-cover', p.variants.every((v) => v.soldOut) && 'opacity-45 grayscale')} />
                  )}
                  <div className="min-w-0 flex-1">
                    <p className="mb-2 truncate font-semibold">{p.nameId}</p>
                    <ul className="flex flex-col gap-2">
                      {p.variants.map((v) => (
                        <li key={v.id} className="flex items-center gap-3">
                          <span className="min-w-0 flex-1 text-sm">
                            <span className="block truncate">{v.nameId}</span>
                            {v.soldOut && v.soldOutAt && (
                              <span className="block text-xs text-muted-foreground">
                                {t('admin.soldOut.since', { time: formatTime(v.soldOutAt) })} ·{' '}
                                {v.soldOutSource === 'cancel_out_of_stock' ? t('admin.soldOut.sourceCancel') : t('admin.soldOut.sourceManual')}
                              </span>
                            )}
                          </span>
                          {v.soldOut ? <SoldOutBadge /> : <span className="text-xs font-semibold text-muted-foreground">{t('admin.soldOut.available')}</span>}
                          <Switch
                            aria-label={`${p.nameId} ${v.nameId}`}
                            checked={!v.soldOut}
                            disabled={toggle.isPending}
                            onCheckedChange={(available) => toggle.mutate({ id: v.id!, soldOut: !available })}
                          />
                        </li>
                      ))}
                    </ul>
                  </div>
                </Card>
              ))}
            </div>
          </section>
        );
      })}
    </>
  );
}
