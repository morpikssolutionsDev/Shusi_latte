// OP-01: scan → free / occupied (Rejoin prompt) / unavailable → name, Dietary Restriction,
// consent → splash on the scan that opened the Bill (M1).
import { api, idempotencyKey, unwrap } from '@sushi-latte/api-client';
import { Button, Logo, cn, dietaryIcons } from '@sushi-latte/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { CheckIcon, HandIcon, LoaderIcon } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useEffect, useId, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { menuQuery, problemOf, tableQuery } from './api';
import { BottomSheet, ease, useName } from './kit';
import { setSession } from './session';
import { LanguageToggle } from './shell';

type GuestSession = { token: string; customerId: string; billId: string; name: string; firstScan: boolean };

export function JoinFlow({ code, onJoined }: { code: string; onJoined: (firstScan: boolean) => void }) {
  const { t } = useTranslation();
  const table = useQuery(tableQuery(code));
  const [step, setStep] = useState<'prompt' | 'form' | 'not-mine'>('prompt');

  const done = (s: GuestSession) => {
    setSession({ token: s.token, customerId: s.customerId, billId: s.billId, name: s.name, code });
    onJoined(s.firstScan);
  };
  const rejoin = useMutation({
    mutationFn: async (customerId: string) =>
      unwrap(await api.POST('/public/tables/{code}/rejoin', { params: { path: { code } }, body: { customerId } })),
    onSuccess: done,
  });
  const notMine = useMutation({
    mutationFn: async () => unwrap(await api.POST('/public/tables/{code}/not-mine', { params: { path: { code } } })),
    onSuccess: () => setStep('not-mine'),
  });

  if (table.isPending) return <CenteredLoader />;
  if (table.isError || table.data.state === 'unavailable') {
    return <Notice title={t('scan.unavailableTitle')} body={t('scan.unavailableBody')} />;
  }
  if (step === 'not-mine') return <Notice icon={<HandIcon className="size-8" aria-hidden />} title={t('occupied.notMineTitle')} body={t('occupied.notMineBody')} />;

  const label = table.data.table?.label ?? '';
  const occupied = table.data.state === 'occupied' && step === 'prompt';
  return (
    <>
      <NameForm code={code} tableLabel={label} onJoined={done} />
      {/* BR-CM-12: plain sheet, no decoration (M12, CONTENT.md rule 6). */}
      <BottomSheet open={occupied} onClose={() => setStep('form')} title={label}>
        <p className="mb-4 text-base">
          {t('occupied.title', { names: table.data.customers.map((c) => c.name).join(', ') })}
        </p>
        <div className="flex flex-col gap-2">
          {table.data.customers.map((c) => (
            <Button key={c.id} size="lg" variant="secondary" className="justify-start" disabled={rejoin.isPending} onClick={() => rejoin.mutate(c.id)}>
              {t('occupied.imName', { name: c.name })}
            </Button>
          ))}
          <Button size="lg" className="mt-2" onClick={() => setStep('form')}>{t('occupied.joining')}</Button>
          <Button size="lg" variant="ghost" disabled={notMine.isPending} onClick={() => notMine.mutate()}>{t('occupied.notMine')}</Button>
        </div>
        {rejoin.isError && <p className="mt-3 text-sm text-[var(--sl-color-feedback-danger-fg)]">{problemOf(rejoin.error)?.message ?? t('errors.generic')}</p>}
      </BottomSheet>
    </>
  );
}

function NameForm({ code, tableLabel, onJoined }: { code: string; tableLabel: string; onJoined: (s: GuestSession) => void }) {
  const { t } = useTranslation();
  const { of } = useName();
  const menu = useQuery(menuQuery);
  const nameId = useId();
  const [name, setName] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [note, setNote] = useState('');
  const [consent, setConsent] = useState(false);
  // One key per distinct submission: a double tap replays, a corrected form is a new request.
  const key = useMemo(idempotencyKey, [name, tags, note, consent]);
  const wantsDietary = tags.length > 0 || note.trim() !== '';
  const join = useMutation({
    mutationFn: async () =>
      unwrap(await api.POST('/public/tables/{code}/join', {
        params: { path: { code } },
        headers: { 'Idempotency-Key': key },
        body: { name, restrictionTagIds: tags, dietaryNote: note, consent },
      })),
    onSuccess: onJoined,
  });
  const blocked = !name.trim() || (wantsDietary && !consent) || join.isPending;

  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col px-4 pb-10 pt-4">
      <div className="flex items-center justify-between">
        <span className="rounded-full bg-sunken px-3 py-1 text-sm font-semibold">{tableLabel}</span>
        <LanguageToggle />
      </div>
      <Logo variant="lockup" animated className="mx-auto mt-6 w-64" />
      <p className="mt-1 text-center text-sm text-text-secondary">{t('join.tagline')}</p>
      <form
        className="mt-8 flex flex-col gap-6"
        onSubmit={(e) => {
          e.preventDefault();
          if (!blocked) join.mutate();
        }}
      >
        <div>
          <h1 className="font-display text-xl font-bold">{t('join.title')}</h1>
          <label htmlFor={nameId} className="mt-3 block text-sm font-semibold">{t('join.name')}</label>
          <input
            id={nameId}
            value={name}
            maxLength={40}
            autoComplete="name"
            onChange={(e) => setName(e.target.value)}
            placeholder={t('join.namePlaceholder')}
            className="mt-1.5 h-13 w-full rounded-full border border-input bg-card px-5 text-base outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
          <p className="mt-1.5 text-xs text-text-secondary">{t('join.nameHint')}</p>
        </div>

        {/* RestrictionPicker (COMPONENTS.md §2): every Dietary Tag as a toggle chip. */}
        <fieldset>
          <legend className="font-semibold">{t('join.restrictionTitle')}</legend>
          <p className="text-xs text-text-secondary">{t('join.restrictionHint')}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            {menu.data?.dietaryTags.map((tag) => {
              const on = tags.includes(tag.id!);
              const Icon = dietaryIcons[tag.code];
              return (
                <button
                  key={tag.id}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setTags(on ? tags.filter((x) => x !== tag.id) : [...tags, tag.id!])}
                  className={cn(
                    'inline-flex h-11 items-center gap-2 rounded-full border px-4 text-sm font-semibold transition-colors',
                    on ? 'border-[var(--sl-color-dietary-warning-border)] bg-[var(--sl-color-dietary-warning-bg)] text-[var(--sl-color-dietary-warning-fg)]' : 'border-border bg-card',
                  )}
                >
                  {on ? <CheckIcon className="size-4" aria-hidden /> : Icon && <Icon className="size-4" aria-hidden />}
                  {of(tag)}
                </button>
              );
            })}
          </div>
          <label className="mt-4 block text-sm font-semibold">
            {t('join.noteLabel')}
            <textarea
              value={note}
              maxLength={300}
              rows={2}
              onChange={(e) => setNote(e.target.value)}
              placeholder={t('join.notePlaceholder')}
              className="mt-1.5 w-full rounded-[20px] border border-input bg-card px-4 py-3 text-base font-normal outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
          <AnimatePresence>
            {wantsDietary && (
              <motion.label
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                transition={ease.expressive}
                className="mt-3 flex items-start gap-3 overflow-hidden rounded-[20px] bg-sunken p-4 text-sm"
              >
                <input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} className="mt-0.5 size-5 shrink-0 accent-[var(--sl-color-brand-salmon-deep)]" />
                <span>{t('join.consent')}</span>
              </motion.label>
            )}
          </AnimatePresence>
          {wantsDietary && !consent && <p className="mt-2 text-xs text-text-secondary">{t('join.consentNeeded')}</p>}
        </fieldset>

        {join.isError && <p className="text-sm text-[var(--sl-color-feedback-danger-fg)]">{problemOf(join.error)?.detail ?? problemOf(join.error)?.message ?? t('errors.generic')}</p>}
        <Button type="submit" size="xl" disabled={blocked} className="w-full">
          {join.isPending && <LoaderIcon className="animate-spin" aria-hidden />}
          {t('join.start')}
        </Button>
      </form>
    </main>
  );
}

/**
 * M1: animated logo splash on the scan that opened the Bill, ≤ 1.5 s, tap to skip; a Rejoin
 * never shows it.
 */
export function Splash({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  useEffect(() => {
    const timer = setTimeout(onDone, reduce ? 600 : 1500);
    return () => clearTimeout(timer);
  }, [onDone, reduce]);
  return (
    <motion.button
      type="button"
      onClick={onDone}
      className="fixed inset-0 z-40 flex flex-col items-center justify-center gap-4 bg-background"
      initial={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={ease.slow}
      aria-label={t('splash.skip')}
      data-testid="splash"
    >
      <Logo variant="lockup" animated={!reduce} className="w-72" />
      <span className="text-sm text-text-secondary">{t('join.tagline')}</span>
    </motion.button>
  );
}

export function Notice({ title, body, icon, action }: { title: string; body: string; icon?: React.ReactNode; action?: React.ReactNode }) {
  return (
    <main className="mx-auto flex min-h-dvh max-w-[480px] flex-col items-center justify-center gap-4 px-6 text-center">
      {icon ? <div className="grid size-16 place-items-center rounded-full bg-sunken text-text-secondary">{icon}</div> : <Logo className="w-24" />}
      <h1 className="font-display text-xl font-bold">{title}</h1>
      <p className="text-text-secondary">{body}</p>
      {action}
    </main>
  );
}

export function CenteredLoader() {
  return (
    <div className="grid min-h-dvh place-items-center">
      <LoaderIcon className="size-8 animate-spin text-text-secondary" aria-label="…" />
    </div>
  );
}
