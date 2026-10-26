import { api, unwrap } from '@sushi-latte/api-client';
import { Button, Card, Chip, Dialog, Field, Input, Select, Switch, cn, imageUrl } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { KeyRoundIcon, PlusIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { crewQuery, useSave, type CrewInput, type CrewMember } from './api';
import { PageHeader } from './layout';

const STATIONS = ['waiter', 'cashier', 'kitchen'] as const;

/** Initials avatar on a brand tile picked from the Crew ID (ICONOGRAPHY.md §3.1). */
export function Avatar({ member, size = 'size-11' }: { member: Pick<CrewMember, 'fullName' | 'crewId' | 'imageKey'>; size?: string }) {
  if (member.imageKey) return <img src={imageUrl(member.imageKey, 'sq320')} alt="" className={cn(size, 'shrink-0 rounded-full object-cover')} />;
  const initials = member.fullName.split(/\s+/).map((w) => w[0]).slice(0, 2).join('').toUpperCase();
  const tiles = ['var(--sl-color-nori-900)', 'var(--sl-color-espresso-700)', 'var(--sl-color-salmon-600)'];
  const tile = tiles[member.crewId.charCodeAt(member.crewId.length - 1) % tiles.length];
  return (
    <span aria-hidden className={cn(size, 'grid shrink-0 place-items-center rounded-full font-display text-sm font-semibold text-white')} style={{ background: tile }}>
      {initials}
    </span>
  );
}

export function CrewPage() {
  const { t } = useTranslation();
  const { data } = useQuery(crewQuery);
  const [edit, setEdit] = useState<(CrewInput & { id?: string }) | null>(null);
  const [resetFor, setResetFor] = useState<CrewMember | null>(null);
  const [newPin, setNewPin] = useState('');
  const save = useSave(
    async ({ id, ...body }: CrewInput & { id?: string }) =>
      id ? unwrap(await api.PUT('/crew/{id}', { params: { path: { id } }, body })) : unwrap(await api.POST('/crew', { body })),
    [['crew']],
    t('admin.common.saved'),
  );
  const resetPin = useSave(
    async ({ id, pin }: { id: string; pin: string }) => unwrap(await api.POST('/crew/{id}/reset-pin', { params: { path: { id } }, body: { pin } })),
    [['crew']],
    t('admin.crew.pinReset'),
  );
  if (!data) return null;

  return (
    <>
      <PageHeader
        title={t('admin.crew.title')}
        actions={<Button onClick={() => setEdit({ crewId: '', fullName: '', role: 'crew', stations: ['waiter'], active: true, pin: '' })}>
          <PlusIcon aria-hidden />{t('admin.crew.add')}
        </Button>}
      />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {data.map((m) => (
          <Card key={m.id} className={cn('flex items-center gap-3 p-3', !m.active && 'opacity-60')}>
            <Avatar member={m} />
            <button type="button" className="min-w-0 flex-1 text-left" onClick={() => setEdit({ id: m.id, crewId: m.crewId, fullName: m.fullName, role: m.role, stations: m.stations, active: m.active, imageKey: m.imageKey })}>
              <span className="block truncate font-semibold">{m.fullName}</span>
              <span className="block truncate text-xs text-muted-foreground">
                <span className="font-mono">{m.crewId}</span> · {[t(`roles.${m.role}`), ...m.stations.map((s) => t(`stations.${s}`))].join(' · ')}
              </span>
              <span className="mt-1 flex gap-1.5">
                {!m.active && <Chip tone="cancelled">{t('admin.common.inactive')}</Chip>}
                {m.lockedUntil && <Chip feedback="danger">{t('admin.crew.locked')}</Chip>}
              </span>
            </button>
            <Button variant="ghost" size="icon" aria-label={`${t('admin.crew.resetPin')} ${m.fullName}`} onClick={() => { setResetFor(m); setNewPin(''); }}>
              <KeyRoundIcon aria-hidden />
            </Button>
          </Card>
        ))}
      </div>

      {edit && (
        <Dialog open onOpenChange={(o) => !o && setEdit(null)} title={edit.id ? edit.fullName : t('admin.crew.add')}
          footer={<>
            <Button variant="secondary" onClick={() => setEdit(null)}>{t('admin.common.cancel')}</Button>
            <Button disabled={save.isPending} onClick={() => save.mutate(edit, { onSuccess: () => setEdit(null) })}>{t('admin.common.save')}</Button>
          </>}>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t('admin.crew.crewId')} htmlFor="c-id" hint={t('admin.crew.crewIdHint')}>
              <Input id="c-id" className="font-mono uppercase" disabled={!!edit.id} value={edit.crewId} onChange={(e) => setEdit({ ...edit, crewId: e.target.value.toUpperCase() })} />
            </Field>
            <Field label={t('admin.crew.fullName')} htmlFor="c-name">
              <Input id="c-name" value={edit.fullName} onChange={(e) => setEdit({ ...edit, fullName: e.target.value })} />
            </Field>
            <Field label={t('admin.crew.role')} htmlFor="c-role">
              <Select id="c-role" value={edit.role} onChange={(e) => setEdit({ ...edit, role: e.target.value as CrewInput['role'], stations: e.target.value === 'crew' ? ['waiter'] : [] })}>
                {(['management', 'outlet_admin', 'crew'] as const).map((r) => <option key={r} value={r}>{t(`roles.${r}`)}</option>)}
              </Select>
            </Field>
            {!edit.id && (
              <Field label={t('admin.crew.pin')} htmlFor="c-pin">
                <Input id="c-pin" inputMode="numeric" maxLength={6} className="font-mono tabular" value={edit.pin ?? ''} onChange={(e) => setEdit({ ...edit, pin: e.target.value.replace(/\D/g, '') })} />
              </Field>
            )}
            {edit.role === 'crew' && (
              <fieldset className="sm:col-span-2">
                <legend className="mb-1.5 text-sm font-semibold">{t('admin.crew.stations')}</legend>
                <div className="flex flex-wrap gap-1.5">
                  {STATIONS.map((s) => {
                    const on = edit.stations.includes(s);
                    return (
                      <button key={s} type="button" aria-pressed={on}
                        onClick={() => setEdit({ ...edit, stations: on ? edit.stations.filter((x) => x !== s) : [...edit.stations, s] })}
                        className={cn('h-9 rounded-chip border px-3 text-sm font-semibold', on ? 'border-foreground bg-foreground text-background' : 'border-border bg-card')}>
                        {t(`stations.${s}`)}
                      </button>
                    );
                  })}
                </div>
              </fieldset>
            )}
            {edit.id && (
              <label className="flex items-center gap-3 text-sm font-semibold sm:col-span-2">
                <Switch checked={edit.active} onCheckedChange={(active) => setEdit({ ...edit, active })} />
                {t('admin.common.active')}
              </label>
            )}
          </div>
        </Dialog>
      )}

      {resetFor && (
        <Dialog open onOpenChange={(o) => !o && setResetFor(null)} title={`${t('admin.crew.resetPin')} · ${resetFor.fullName}`}
          footer={<>
            <Button variant="secondary" onClick={() => setResetFor(null)}>{t('admin.common.cancel')}</Button>
            <Button disabled={newPin.length !== 6 || resetPin.isPending} onClick={() => resetPin.mutate({ id: resetFor.id, pin: newPin }, { onSuccess: () => setResetFor(null) })}>
              {t('admin.crew.resetPin')}
            </Button>
          </>}>
          <Field label={t('admin.crew.newPin')} htmlFor="new-pin">
            <Input id="new-pin" inputMode="numeric" maxLength={6} className="font-mono tabular" value={newPin} onChange={(e) => setNewPin(e.target.value.replace(/\D/g, ''))} />
          </Field>
        </Dialog>
      )}
    </>
  );
}
