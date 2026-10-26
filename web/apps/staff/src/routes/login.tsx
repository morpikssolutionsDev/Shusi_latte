import { api, ApiError, unwrap, type DemoAccount } from '@sushi-latte/api-client';
import { Button, Card, Input, Label, Logo, PinPad, cn, imageUrl } from '@sushi-latte/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { healthQuery } from '../session';



export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [crewId, setCrewId] = useState('');
  const [pin, setPin] = useState('');
  const [shake, setShake] = useState(0);
  const health = useQuery(healthQuery);
  const demo = useQuery({
    queryKey: ['demo-accounts'],
    queryFn: async () => unwrap(await api.GET('/demo/accounts')),
    enabled: health.data?.demo === true,
  });

  const login = useMutation({
    mutationFn: async (vars: { crewId: string; pin: string }) =>
      unwrap(await api.POST('/auth/login', { body: { crewId: vars.crewId.trim().toUpperCase(), pin: vars.pin } })),
    onSuccess: (me) => {
      queryClient.setQueryData(['me'], me);
      void navigate({ to: '/' });
    },
    onError: () => {
      setPin('');
      setShake((n) => n + 1);
    },
  });

  const submit = (value = pin) => {
    if (crewId.trim() && value.length === 6) login.mutate({ crewId, pin: value });
  };

  return (
    <div className="mx-auto grid min-h-[calc(100dvh-28px)] w-full max-w-5xl items-center gap-8 px-4 py-8 md:grid-cols-[1fr_minmax(0,22rem)]">
      <Card className="mx-auto flex w-full max-w-sm flex-col items-center gap-6 p-6 shadow-lg md:col-start-1">
        <Logo variant="lockup" animated className="w-64" />
        <form
          className="flex w-full flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <Label htmlFor="crew-id">{t('login.crewId')}</Label>
          <Input
            id="crew-id"
            value={crewId}
            onChange={(e) => setCrewId(e.target.value)}
            placeholder={t('login.crewIdPlaceholder')}
            autoComplete="username"
            autoCapitalize="characters"
            className="font-mono uppercase placeholder:font-sans placeholder:normal-case"
          />
        </form>
        <PinPad
          key={shake}
          value={pin}
          onChange={setPin}
          onComplete={submit}
          error={shake > 0}
          disabled={login.isPending}
          label={t('login.pin')}
        />
        <p role="alert" className="min-h-5 text-center text-sm font-semibold text-destructive">
          {login.error ? errorMessage(login.error, t) : ''}
        </p>
        <Button size="lg" className="w-full" disabled={!crewId || pin.length < 6 || login.isPending} onClick={() => submit()}>
          {t('login.submit')}
        </Button>
      </Card>

      {demo.data && <DemoAccounts accounts={demo.data} selected={crewId} onPick={(a) => { setCrewId(a.crewId); setPin(''); }} />}
    </div>
  );
}

function DemoAccounts({ accounts, selected, onPick }: { accounts: DemoAccount[]; selected: string; onPick: (a: DemoAccount) => void }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="demo-title" className="flex flex-col gap-3">
      <div>
        <h2 id="demo-title" className="font-display text-lg font-semibold">{t('login.demoTitle')}</h2>
        <p className="text-sm text-muted-foreground">{t('login.demoHint')}</p>
      </div>
      <ul className="grid max-h-[70dvh] grid-cols-1 gap-2 overflow-y-auto pr-1 sm:grid-cols-2 md:grid-cols-1">
        {accounts.map((a) => (
          <li key={a.crewId}>
            <button
              type="button"
              onClick={() => onPick(a)}
              aria-pressed={selected === a.crewId}
              className={cn(
                'flex w-full items-center gap-3 rounded-card border bg-card p-2 text-left transition-colors hover:bg-accent',
                selected === a.crewId ? 'border-ring' : 'border-border-subtle',
              )}
            >
              {a.imageKey ? <img src={imageUrl(a.imageKey, 'sq320')} alt="" className="size-11 rounded-full bg-sunken object-cover" /> : <span className="size-11 rounded-full bg-sunken" />}
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold">{a.name}</span>
                <span className="block truncate text-xs text-muted-foreground">
                  {[t(`roles.${a.role}`), ...a.stations.map((s) => t(`stations.${s}`))].join(' · ')}
                </span>
                {a.note && <span className="block truncate text-xs text-text-secondary">{a.note}</span>}
              </span>
              <span className="text-right font-mono text-xs leading-tight">
                <span className="block font-semibold">{a.crewId}</span>
                <span className="block text-muted-foreground">{a.pin}</span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function errorMessage(error: Error, t: (k: string, o?: Record<string, unknown>) => string): string {
  if (!(error instanceof ApiError)) return t('login.network');
  switch (error.code) {
    case 'invalid_credentials':
      return typeof error.extra.attemptsRemaining === 'number'
        ? t('login.wrongPin', { count: error.extra.attemptsRemaining })
        : t('login.wrongCredentials');
    case 'account_locked': {
      const until = new Date(Number(error.extra.lockedUntil));
      return t('login.locked', { time: until.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }) });
    }
    case 'rate_limited':
      return t('login.rateLimited');
    default:
      return error.message;
  }
}
