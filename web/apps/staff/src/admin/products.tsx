import type { Principal } from '@sushi-latte/api-client';
import { Button, Card, Chip, Input, SoldOutBadge, cn, formatRupiah, imageUrl } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { ImageOffIcon, PlusIcon, SearchIcon } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { catalogQuery, minutes, type Product } from './api';
import { PageHeader } from './layout';

export function ProductsPage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data } = useQuery(catalogQuery);
  const [categoryId, setCategoryId] = useState<string>('all');
  const [search, setSearch] = useState('');
  const canEdit = me.role === 'management';

  const grouped = useMemo(() => {
    if (!data) return [];
    const q = search.trim().toLowerCase();
    return data.categories
      .filter((c) => categoryId === 'all' || c.id === categoryId)
      .map((c) => ({
        category: c,
        products: data.products.filter(
          (p) => p.categoryId === c.id && (!q || p.nameId.toLowerCase().includes(q) || p.nameEn.toLowerCase().includes(q)),
        ),
      }))
      .filter((g) => g.products.length > 0);
  }, [data, categoryId, search]);

  return (
    <>
      <PageHeader
        title={t('admin.products.title')}
        actions={
          canEdit && (
            <Button onClick={() => void navigate({ to: '/admin/products/$productId', params: { productId: 'new' } })}>
              <PlusIcon aria-hidden />
              {t('admin.products.add')}
            </Button>
          )
        }
      />
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-xs">
          <SearchIcon aria-hidden className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            aria-label={t('admin.common.search')}
            placeholder={t('admin.products.searchPlaceholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <div role="tablist" className="flex flex-wrap gap-1.5">
          {[{ id: 'all', nameId: t('admin.common.all') }, ...(data?.categories ?? [])].map((c) => (
            <button
              key={c.id}
              role="tab"
              aria-selected={categoryId === c.id}
              onClick={() => setCategoryId(c.id!)}
              className={cn(
                'h-9 rounded-chip border px-3 text-sm font-semibold',
                categoryId === c.id ? 'border-foreground bg-foreground text-background' : 'border-border bg-card hover:bg-accent',
              )}
            >
              {c.nameId}
            </button>
          ))}
        </div>
      </div>

      {grouped.map(({ category, products }) => (
        <section key={category.id} className="mb-8" aria-labelledby={`cat-${category.id}`}>
          <h2 id={`cat-${category.id}`} className="mb-3 text-xs font-bold uppercase tracking-[0.12em] text-muted-foreground">
            {category.nameId}
          </h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {products.map((p) => (
              <ProductRow key={p.id} product={p} onOpen={() => void navigate({ to: '/admin/products/$productId', params: { productId: p.id! } })} />
            ))}
          </div>
        </section>
      ))}
    </>
  );
}

function ProductRow({ product: p, onOpen }: { product: Product; onOpen: () => void }) {
  const { t } = useTranslation();
  const soldOut = p.variants.filter((v) => v.soldOut).length;
  const allSoldOut = soldOut > 0 && soldOut === p.variants.length;
  const prices = p.variants.map((v) => v.price);
  const min = Math.min(...prices);
  const max = Math.max(...prices);
  return (
    <Card className={cn('p-0', p.status !== 'active' && 'border-dashed')}>
      <button type="button" onClick={onOpen} className="flex w-full items-center gap-3 p-3 text-left">
        <div className="relative size-20 shrink-0 overflow-hidden rounded-lg bg-sunken">
          {p.imageKey ? (
            <img src={imageUrl(p.imageKey, 'sq320')} alt="" className={cn('size-full object-cover', allSoldOut && 'opacity-45 grayscale')} loading="lazy" />
          ) : (
            <span className="grid size-full place-items-center text-muted-foreground" title={t('admin.products.noPhoto')}>
              <ImageOffIcon aria-hidden className="size-6" />
            </span>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{p.nameId}</p>
          <p className="tabular text-sm font-bold">
            {min === max ? formatRupiah(min) : `${formatRupiah(min)} – ${formatRupiah(max)}`}
          </p>
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            {p.status !== 'active' && <Chip tone={p.status === 'archived' ? 'cancelled' : 'served'}>{t(`admin.status.${p.status}`)}</Chip>}
            {allSoldOut ? <SoldOutBadge /> : soldOut > 0 && <Chip feedback="warning">{t('admin.products.soldOutCount', { count: soldOut })}</Chip>}
            <span className="text-xs text-muted-foreground">
              {t('admin.products.variants', { count: p.variants.length })} · {t('admin.products.prep', { minutes: minutes(p.prepTimeSeconds) })}
            </span>
          </div>
        </div>
      </button>
    </Card>
  );
}
