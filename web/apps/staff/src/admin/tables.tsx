import { api, unwrap } from '@sushi-latte/api-client';
import { Button, Card, Dialog, Field, Input, Switch, cn } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { PlusIcon, PrinterIcon, RefreshCwIcon } from 'lucide-react';
import QRCode from 'qrcode';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { tablesQuery, useSave, type Table } from './api';
import { PageHeader } from './layout';

/** The Client Menu entry point for a Table (BR-CM-01). Served at the site root. */
export const tableUrl = (token: string) => `${import.meta.env.VITE_MENU_ORIGIN ?? location.origin}/t/${token}`;

function useQr(token: string) {
  const [src, setSrc] = useState('');
  useEffect(() => {
    void QRCode.toDataURL(tableUrl(token), { margin: 1, width: 480, color: { dark: '#1E2B25', light: '#FFFFFF' } }).then(setSrc);
  }, [token]);
  return src;
}

export function TablesPage() {
  const { t } = useTranslation();
  const { data } = useQuery(tablesQuery);
  const [edit, setEdit] = useState<Table | null>(null);
  const [regen, setRegen] = useState<Table | null>(null);
  const save = useSave(
    async (x: Table) => (x.id ? unwrap(await api.PUT('/tables/{id}', { params: { path: { id: x.id } }, body: x })) : unwrap(await api.POST('/tables', { body: x }))),
    [['tables']],
    t('admin.common.saved'),
  );
  const regenerate = useSave(
    async (id: string) => unwrap(await api.POST('/tables/{id}/regenerate-qr', { params: { path: { id } } })),
    [['tables']],
    t('admin.tables.regenerated'),
  );
  if (!data) return null;
  const nextNumber = Math.max(0, ...data.map((x) => x.number)) + 1;

  return (
    <>
      <PageHeader
        title={t('admin.tables.title')}
        actions={
          <>
            <Button variant="secondary" onClick={() => printCards(data.filter((x) => x.active), t('admin.tables.scanToOrder'))}>
              <PrinterIcon aria-hidden />
              {t('admin.tables.print')}
            </Button>
            <Button onClick={() => setEdit({ number: nextNumber, label: `Meja ${nextNumber}`, active: true })}>
              <PlusIcon aria-hidden />
              {t('admin.tables.add')}
            </Button>
          </>
        }
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
        {data.map((x) => (
          <TableCard key={x.id} table={x} onEdit={() => setEdit(x)} onRegenerate={() => setRegen(x)}
            onToggle={(active) => save.mutate({ ...x, active })} />
        ))}
      </div>

      {edit && (
        <Dialog open onOpenChange={(o) => !o && setEdit(null)} title={edit.id ? edit.label : t('admin.tables.add')}
          footer={<>
            <Button variant="secondary" onClick={() => setEdit(null)}>{t('admin.common.cancel')}</Button>
            <Button disabled={save.isPending} onClick={() => save.mutate(edit, { onSuccess: () => setEdit(null) })}>{t('admin.common.save')}</Button>
          </>}>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t('admin.tables.number')} htmlFor="t-num">
              <Input id="t-num" type="number" min={1} className="tabular" value={edit.number} onChange={(e) => setEdit({ ...edit, number: Number(e.target.value) })} />
            </Field>
            <Field label={t('admin.tables.label')} htmlFor="t-label">
              <Input id="t-label" value={edit.label} onChange={(e) => setEdit({ ...edit, label: e.target.value })} />
            </Field>
          </div>
        </Dialog>
      )}
      {regen && (
        <Dialog open onOpenChange={(o) => !o && setRegen(null)} title={t('admin.tables.regenerate')}
          description={t('admin.tables.regenerateConfirm', { label: regen.label })}
          footer={<>
            <Button variant="secondary" onClick={() => setRegen(null)}>{t('admin.common.cancel')}</Button>
            <Button variant="danger" disabled={regenerate.isPending} onClick={() => regenerate.mutate(regen.id!, { onSuccess: () => setRegen(null) })}>
              {t('admin.tables.regenerate')}
            </Button>
          </>}>
          <p className="text-sm">{tableUrl(regen.qrToken ?? '')}</p>
        </Dialog>
      )}
    </>
  );
}

function TableCard({ table, onEdit, onRegenerate, onToggle }: { table: Table; onEdit: () => void; onRegenerate: () => void; onToggle: (active: boolean) => void }) {
  const { t } = useTranslation();
  const qr = useQr(table.qrToken ?? '');
  return (
    <Card className={cn('flex flex-col gap-3 p-4', !table.active && 'opacity-60')}>
      <div className="flex items-center justify-between gap-2">
        <button type="button" onClick={onEdit} className="text-left text-lg font-bold hover:underline">{table.label}</button>
        <Switch aria-label={`${table.label} ${t('admin.common.active')}`} checked={table.active} onCheckedChange={onToggle} />
      </div>
      {qr && <img src={qr} alt={`${t('admin.tables.qr')} ${table.label}`} className="aspect-square w-full rounded-lg border border-border-subtle" />}
      <p className="truncate font-mono text-xs text-muted-foreground" title={tableUrl(table.qrToken ?? '')}>/t/{table.qrToken}</p>
      <Button variant="ghost" size="sm" onClick={onRegenerate}>
        <RefreshCwIcon aria-hidden />
        {t('admin.tables.regenerate')}
      </Button>
    </Card>
  );
}

/** Opens a print-friendly sheet of table tent cards. */
async function printCards(tables: Table[], caption: string) {
  const cards = await Promise.all(
    tables.map(async (x) => ({ label: x.label, qr: await QRCode.toDataURL(tableUrl(x.qrToken ?? ''), { margin: 1, width: 600 }) })),
  );
  const w = window.open('', '_blank');
  if (!w) return;
  w.document.write(`<!doctype html><title>QR Meja</title><style>
    body{font-family:'Plus Jakarta Sans',system-ui,sans-serif;margin:0;display:grid;grid-template-columns:repeat(2,1fr);gap:12mm;padding:12mm}
    .card{border:1px solid #D5DAD8;border-radius:6mm;padding:8mm;text-align:center;break-inside:avoid}
    h1{font-size:22pt;margin:0 0 4mm;color:#1E2B25} p{margin:4mm 0 0;color:#545B58;font-size:12pt} img{width:100%}
  </style>${cards.map((c) => `<div class="card"><h1>${c.label}</h1><img src="${c.qr}" alt=""><p>${caption}</p></div>`).join('')}`);
  w.document.close();
  w.onload = () => w.print();
}
