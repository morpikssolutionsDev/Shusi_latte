// Waiter module shell: 8″ portrait tablet, one hand, bottom tab bar (LAYOUT.md §3).
import type { Principal } from '@sushi-latte/api-client';
import { cn, domainIcons } from '@sushi-latte/ui';
import { useQuery } from '@tanstack/react-query';
import { Link, Outlet, useRouterState } from '@tanstack/react-router';
import { ArrowLeftIcon, BellIcon, ClipboardListIcon, Volume2Icon, VolumeXIcon } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { enableSound, feedQuery, soundEnabled, useWaiterRealtime } from './api';

export function WaiterLayout({ me }: { me: Principal }) {
  const { t } = useTranslation();
  const status = useWaiterRealtime();
  const feed = useQuery(feedQuery);
  // Compare paths without the /staff basepath, whether or not the router includes it.
  const path = useRouterState({ select: (s) => s.location.pathname.replace(/^\/staff(?=\/|$)/, '') || '/' });
  const [sound, setSound] = useState(soundEnabled());
  const waiting = (feed.data?.unclaimed.length ?? 0) + (feed.data?.calls.length ?? 0);
  const ready = feed.data?.mine.reduce((n, o) => n + o.items.filter((i) => i.status === 'ready').length, 0) ?? 0;
  const TableIcon = domainIcons.table;
  const tabs = [
    { to: '/waiter', label: t('waiter.tabs.notifications'), icon: BellIcon, badge: waiting + ready, exact: true },
    { to: '/waiter/tables', label: t('waiter.tabs.tables'), icon: TableIcon, badge: 0 },
    { to: '/waiter/orders', label: t('waiter.tabs.orders'), icon: ClipboardListIcon, badge: feed.data?.mine.length ?? 0 },
  ] as const;
  return (
    <div className="mx-auto flex min-h-[calc(100dvh-28px)] max-w-3xl flex-col">
      <header className="sticky top-0 z-20 flex items-center gap-2 border-b border-border-subtle bg-background/95 px-4 py-3 backdrop-blur">
        <Link to="/" className="grid size-10 place-items-center rounded-control hover:bg-accent" aria-label={t('admin.nav.back')}>
          <ArrowLeftIcon className="size-5" aria-hidden />
        </Link>
        <div className="min-w-0 flex-1">
          <p className="font-bold">{t('modules.waiter')}</p>
          <p className="text-xs text-muted-foreground"><span className="font-mono">{me.crewId}</span> · {me.name} · {t(`realtime.${status}`)}</p>
        </div>
        <button
          type="button"
          onClick={() => {
            enableSound();
            setTimeout(() => setSound(soundEnabled()), 50);
          }}
          className={cn('inline-flex h-10 items-center gap-1.5 rounded-control px-3 text-sm font-semibold', sound ? 'text-muted-foreground' : 'bg-[var(--sl-color-feedback-warning-bg)] text-[var(--sl-color-feedback-warning-fg)]')}
        >
          {sound ? <Volume2Icon className="size-4" aria-hidden /> : <VolumeXIcon className="size-4" aria-hidden />}
          {sound ? t('waiter.notify.soundOn') : t('waiter.notify.soundOff')}
        </button>
      </header>
      <main className="flex-1 px-4 pb-28 pt-4">
        <Outlet />
      </main>
      <nav className="fixed inset-x-0 bottom-0 z-20 mx-auto flex max-w-3xl border-t border-border bg-card pb-[env(safe-area-inset-bottom)]" aria-label={t('modules.waiter')}>
        {tabs.map(({ to, label, icon: Icon, badge, ...rest }) => {
          const active = 'exact' in rest ? path === to || path === `${to}/` : path.startsWith(to);
          return (
            <Link key={to} to={to} className={cn('relative flex h-16 flex-1 flex-col items-center justify-center gap-1 text-xs font-semibold', active ? 'text-foreground' : 'text-muted-foreground')}>
              {active && <span className="absolute inset-x-6 top-0 h-0.5 rounded-full bg-foreground" />}
              <span className="relative">
                <Icon className="size-6" aria-hidden />
                {badge > 0 && (
                  <span className="tabular absolute -right-3 -top-2 grid h-5 min-w-5 place-items-center rounded-full bg-[var(--sl-color-status-pending-solid)] px-1 text-[11px] font-bold text-white">{badge}</span>
                )}
              </span>
              {label}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
