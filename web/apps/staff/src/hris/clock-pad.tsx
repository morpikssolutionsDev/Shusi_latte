// ClockInPad (COMPONENTS.md §3) on the shared HRIS tablet (BR-HR-03, TR-AUTH-08): large clock,
// Crew ID + PinPad, a result card, and the clock-out blockers — cash not handed over (BR-HR-07)
// and open Claims to hand over (G-06). The result clears itself for the next person.
import { ApiError } from '@sushi-latte/api-client';
import { Button, Card, Input, Logo, PinPad, cn } from '@sushi-latte/ui';
import { useMutation } from '@tanstack/react-query';
import { CheckCircle2Icon, HandCoinsIcon, LogInIcon, LogOutIcon, TriangleAlertIcon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { errorMessage } from '../routes/login';
import { hris, wibTime, type ClockResult, type OpenClaim } from './api';

type Outcome =
  | { kind: 'done'; result: ClockResult }
  | { kind: 'cash'; message: string }
  | { kind: 'claims'; claims: OpenClaim[] }
  | null;

const RESET_AFTER = 8_000;

function useNow() {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  return now;
}

export function ClockPad() {
  const { t } = useTranslation();
  const now = useNow();
  const [crewId, setCrewId] = useState('');
  const [pin, setPin] = useState('');
  const [shake, setShake] = useState(0);
  const [outcome, setOutcome] = useState<Outcome>(null);
  const [error, setError] = useState('');
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const reset = () => {
    clearTimeout(timer.current);
    setCrewId('');
    setPin('');
    setOutcome(null);
    setError('');
  };
  const later = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(reset, RESET_AFTER);
  };
  useEffect(() => () => clearTimeout(timer.current), []);

  const clock = useMutation({
    mutationFn: (acknowledgeClaims: boolean) => hris.clock({ crewId: crewId.trim().toUpperCase(), pin, acknowledgeClaims }),
    onSuccess: (result) => {
      setOutcome({ kind: 'done', result });
      setError('');
      later();
    },
    onError: (e) => {
      if (e instanceof ApiError && e.code === 'cash_held') {
        setOutcome({ kind: 'cash', message: e.detail ?? e.message });
        later();
      } else if (e instanceof ApiError && e.code === 'open_claims') {
        setOutcome({ kind: 'claims', claims: (e.extra.claims as OpenClaim[]) ?? [] });
      } else {
        setPin('');
        setShake((n) => n + 1);
        setError(errorMessage(e, t));
      }
    },
  });

  const time = new Date(now).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' }).replace(':', '.');
  const date = new Date(now).toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'Asia/Jakarta' });

  return (
    <main className="grid min-h-[calc(100dvh-28px)] place-items-center bg-background p-6">
      <div className="grid w-full max-w-5xl items-center gap-10 md:grid-cols-2">
        <section className="flex flex-col items-center gap-3 text-center md:items-start md:text-left">
          <Logo className="size-16" />
          <p className="tabular font-display text-8xl font-semibold leading-none" aria-live="off">{time}</p>
          <p className="text-xl text-muted-foreground first-letter:uppercase">{date}</p>
          <p className="mt-4 max-w-sm text-muted-foreground">{t('hris.pad.hint')}</p>
        </section>

        <Card className="flex flex-col gap-5 p-6" aria-live="polite">
          {outcome ? (
            <Result outcome={outcome} onContinue={() => clock.mutate(true)} onBack={reset} pending={clock.isPending} />
          ) : (
            <>
              <h1 className="text-2xl font-bold">{t('hris.pad.title')}</h1>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="pad-crew" className="text-sm font-semibold">{t('hris.pad.crewId')}</label>
                <Input id="pad-crew" value={crewId} onChange={(e) => setCrewId(e.target.value)} placeholder={t('login.crewIdPlaceholder')}
                  autoComplete="off" autoCapitalize="characters" className="h-14 font-mono text-2xl uppercase placeholder:font-sans placeholder:text-base placeholder:normal-case" />
              </div>
              <PinPad key={shake} value={pin} onChange={setPin} onComplete={() => crewId.trim() && clock.mutate(false)} error={shake > 0}
                disabled={clock.isPending} label={t('login.pin')} />
              <p role="alert" className="min-h-5 text-center text-sm font-semibold text-destructive">{error}</p>
            </>
          )}
        </Card>
      </div>
    </main>
  );
}

function Result({ outcome, onContinue, onBack, pending }: { outcome: NonNullable<Outcome>; onContinue: () => void; onBack: () => void; pending: boolean }) {
  const { t } = useTranslation();
  if (outcome.kind === 'cash') {
    return (
      <div className="flex flex-col gap-4" data-testid="pad-blocked">
        <h2 className="flex items-center gap-2 text-2xl font-bold text-[var(--sl-color-feedback-danger-fg)]">
          <HandCoinsIcon className="size-7" aria-hidden />{t('hris.pad.cashTitle')}
        </h2>
        <p className="text-xl font-semibold">{outcome.message}</p>
        <Button size="lg" variant="secondary" onClick={onBack}>{t('hris.pad.back')}</Button>
      </div>
    );
  }
  if (outcome.kind === 'claims') {
    return (
      <div className="flex flex-col gap-4" data-testid="pad-claims">
        <h2 className="flex items-center gap-2 text-2xl font-bold text-[var(--sl-color-feedback-warning-fg)]">
          <TriangleAlertIcon className="size-7" aria-hidden />{t('hris.pad.claimsTitle')}
        </h2>
        <p className="text-muted-foreground">{t('hris.pad.claimsHint')}</p>
        <ul className="flex flex-col gap-2">
          {outcome.claims.map((c) => (
            <li key={c.orderId} className="rounded-control border border-border-subtle px-4 py-3 font-semibold">{t('hris.pad.claim', { table: c.table, seq: c.seq })}</li>
          ))}
        </ul>
        <div className="grid grid-cols-2 gap-2">
          <Button size="lg" variant="secondary" onClick={onBack}>{t('hris.pad.back')}</Button>
          <Button size="lg" disabled={pending} onClick={onContinue}>{t('hris.pad.continue')}</Button>
        </div>
      </div>
    );
  }
  const r = outcome.result;
  const isIn = r.action === 'in';
  const Icon = isIn ? LogInIcon : LogOutIcon;
  return (
    <div className="flex flex-col gap-3" data-testid="pad-result">
      <CheckCircle2Icon className="size-12 text-[var(--sl-color-feedback-success-fg)]" aria-hidden />
      <h2 className="text-2xl font-bold">{t(isIn ? 'hris.pad.greetingIn' : 'hris.pad.greetingOut', { name: r.name.split(' ')[0] })}</h2>
      <p className="flex items-center gap-2 text-3xl font-semibold">
        <Icon className="size-7" aria-hidden />
        {t(isIn ? 'hris.pad.in' : 'hris.pad.out', { time: wibTime(r.at).replace(':', '.') })}
      </p>
      <p className="text-lg text-muted-foreground">
        {r.shift ? t('hris.pad.shift', { start: wibTime(r.shift.startsAt).replace(':', '.'), end: wibTime(r.shift.endsAt).replace(':', '.') }) : t('hris.pad.noShift')}
      </p>
      {isIn && r.lateMinutes > 0 && (
        <p className={cn('font-semibold text-[var(--sl-color-feedback-warning-fg)]')}>{t('hris.pad.late', { count: r.lateMinutes })}</p>
      )}
      {!isIn && (
        <p className="font-semibold">{t('hris.pad.worked', { hours: Math.floor(r.workedMinutes / 60), minutes: r.workedMinutes % 60 })}</p>
      )}
      <Button size="lg" variant="secondary" onClick={onBack}>{t('hris.pad.done')}</Button>
    </div>
  );
}
