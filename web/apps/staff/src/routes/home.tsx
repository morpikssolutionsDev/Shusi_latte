import { api, unwrap, type Principal } from '@sushi-latte/api-client';
import { Button, Card, Logo, cn } from '@sushi-latte/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, Navigate, useNavigate, useRouter } from '@tanstack/react-router';
import { ChefHat, ClipboardList, CoffeeIcon, LogOut, MonitorPlay, ReceiptText, Settings2, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { ExitScreenButton } from '../components/exit-screen';
import { useRealtime } from '../realtime';

const SCREENS = [
  { screen: 'kitchen', icon: ChefHat },
  { screen: 'bar', icon: CoffeeIcon },
  { screen: 'display', icon: MonitorPlay },
  { screen: 'hris', icon: Users },
] as const;

const MODULES = [
  { key: 'admin', icon: Settings2, roles: ['management', 'outlet_admin'] },
  { key: 'waiter', icon: ClipboardList, station: 'waiter' },
  { key: 'kitchen', icon: ChefHat, station: 'kitchen' },
  { key: 'cashier', icon: ReceiptText, station: 'cashier' },
  { key: 'hris', icon: Users },
] as const;

/** Where each built module opens. */
function moduleLink(key: (typeof MODULES)[number]['key']) {
  if (key === 'kitchen') return { to: '/kitchen/$area', params: { area: 'kitchen' } } as const;
  const to = { admin: '/admin/products', cashier: '/cashier', waiter: '/waiter', hris: '/hris' } as const;
  return { to: to[key] } as const;
}

function canUse(me: Principal, m: (typeof MODULES)[number]) {
  const admin = me.role === 'management' || me.role === 'outlet_admin';
  if ('roles' in m) return (m.roles as readonly string[]).includes(me.role);
  if ('station' in m) return admin || me.stations.includes(m.station);
  return true;
}

export function HomePage({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const router = useRouter();
  const status = useRealtime(['menu']);

  const logout = useMutation({
    mutationFn: async () => unwrap(await api.POST('/auth/logout')),
    onSettled: () => {
      queryClient.setQueryData(['me'], null);
      void navigate({ to: '/login' });
    },
  });
  const screenMode = useMutation({
    mutationFn: async (screen: (typeof SCREENS)[number]['screen']) =>
      unwrap(await api.POST('/auth/screen-mode', { body: { screen } })),
    onSuccess: (p) => {
      queryClient.setQueryData(['me'], p);
      if (p.screen === 'kitchen' || p.screen === 'bar') void navigate({ to: '/kitchen/$area', params: { area: p.screen } });
      else if (p.screen === 'hris') void navigate({ to: '/clock' });
      else void router.invalidate(); // the home route re-reads the principal from its context
    },
  });

  // A Kitchen or Bar screen goes straight to its display, the HRIS tablet to its pad (TR-AUTH-08).
  if (me.kind === 'screen' && (me.screen === 'kitchen' || me.screen === 'bar')) {
    return <Navigate to="/kitchen/$area" params={{ area: me.screen }} />;
  }
  if (me.kind === 'screen' && me.screen === 'hris') return <Navigate to="/clock" />;
  if (me.kind === 'screen') {
    return (
      <main className="grid min-h-[calc(100dvh-28px)] place-items-center p-6 text-center">
        <div className="flex flex-col items-center gap-4">
          <Logo animated className="w-40" />
          <h1 className="font-display text-2xl font-semibold">{t(`screens.${me.screen}`)}</h1>
          <p className="text-muted-foreground">{t('home.comingSoon')}</p>
          <RealtimeDot status={status} />
          <ExitScreenButton />
        </div>
      </main>
    );
  }

  const isAdmin = me.role === 'management' || me.role === 'outlet_admin';
  return (
    <main className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-6">
      <header className="flex items-center gap-3">
        <Logo className="size-12" />
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-xl font-bold">{t('home.greeting', { name: me.name })}</h1>
          <p className="text-sm text-muted-foreground">
            <span className="font-mono">{me.crewId}</span> ·{' '}
            {[t(`roles.${me.role}`), ...me.stations.map((s) => t(`stations.${s}`))].join(' · ')}
          </p>
        </div>
        <RealtimeDot status={status} />
        <Button variant="secondary" onClick={() => logout.mutate()}>
          <LogOut aria-hidden />
          {t('home.logout')}
        </Button>
      </header>

      <section aria-labelledby="modules" className="flex flex-col gap-3">
        <h2 id="modules" className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('home.modules')}</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {MODULES.filter((m) => canUse(me, m)).map(({ key, icon: Icon }) => (
            <Link key={key} {...moduleLink(key)} className="rounded-card focus-visible:outline-2">
              <Card className="flex h-full flex-col items-start gap-3 shadow-md transition-transform hover:-translate-y-0.5">
                <Icon aria-hidden className="size-6" />
                <span className="font-semibold">{t(`modules.${key}`)}</span>
              </Card>
            </Link>
          ))}
        </div>
      </section>

      {isAdmin && (
        <section aria-labelledby="screens" className="flex flex-col gap-3">
          <div>
            <h2 id="screens" className="text-sm font-bold uppercase tracking-[0.12em] text-muted-foreground">{t('home.screenMode')}</h2>
            <p className="text-sm text-muted-foreground">{t('home.screenModeHint')}</p>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {SCREENS.map(({ screen, icon: Icon }) => (
              <Button key={screen} variant="outline" size="lg" onClick={() => screenMode.mutate(screen)}>
                <Icon aria-hidden />
                {t(`screens.${screen}`)}
              </Button>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

function RealtimeDot({ status }: { status: 'connecting' | 'open' | 'closed' }) {
  const { t } = useTranslation();
  return (
    <span className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground" role="status">
      <span
        aria-hidden
        className={cn(
          'size-2.5 rounded-full',
          status === 'open' && 'bg-[var(--sl-color-status-ready-solid)]',
          status === 'connecting' && 'bg-[var(--sl-color-status-pending-solid)]',
          status === 'closed' && 'bg-[var(--sl-color-status-voided-solid)]',
        )}
      />
      {t(`realtime.${status}`)}
    </span>
  );
}
