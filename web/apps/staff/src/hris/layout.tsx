// HRIS module shell: desktop left rail like the Admin Panel (LAYOUT.md §3). Everyone sees their
// own page and the schedule; attendance and leave decisions are for Outlet Admin and Management;
// employee records for Management (BR-HR-01..05).
import type { Principal } from '@sushi-latte/api-client';
import { Logo, cn } from '@sushi-latte/ui';
import { Link, Outlet } from '@tanstack/react-router';
import { ArrowLeftIcon, CalendarDaysIcon, ClipboardCheckIcon, IdCardIcon, PlaneIcon, UserIcon, type LucideIcon } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useHrisRealtime } from './api';

type NavItem = { to: string; key: string; icon: LucideIcon; who: 'all' | 'admin' | 'management' };

const NAV: NavItem[] = [
  { to: '/hris', key: 'me', icon: UserIcon, who: 'all' },
  { to: '/hris/attendance', key: 'attendance', icon: ClipboardCheckIcon, who: 'admin' },
  { to: '/hris/schedule', key: 'schedule', icon: CalendarDaysIcon, who: 'all' },
  { to: '/hris/leave', key: 'leave', icon: PlaneIcon, who: 'admin' },
  { to: '/hris/employees', key: 'employees', icon: IdCardIcon, who: 'management' },
];

export const isHrAdmin = (me: Principal) => me.role === 'management' || me.role === 'outlet_admin';

export function HrisLayout({ me }: { me: Principal }) {
  const { t } = useTranslation();
  useHrisRealtime();
  useEffect(() => {
    const root = document.documentElement;
    root.dataset.surface = 'admin';
    return () => {
      root.dataset.surface = 'staff';
    };
  }, []);
  const items = NAV.filter((n) => n.who === 'all' || (n.who === 'admin' ? isHrAdmin(me) : me.role === 'management'));
  return (
    <div className="flex min-h-[calc(100dvh-28px)] flex-col lg:flex-row">
      <nav aria-label={t('hris.nav.title')} className="border-border-subtle bg-card lg:sticky lg:top-0 lg:h-dvh lg:w-62 lg:shrink-0 lg:border-r">
        <div className="hidden items-center gap-2 px-5 py-5 lg:flex">
          <Logo className="size-10" />
          <span className="font-display text-base font-semibold">{t('hris.nav.title')}</span>
        </div>
        <ul className="flex gap-1 overflow-x-auto border-b border-border-subtle px-2 py-2 lg:flex-col lg:border-b-0 lg:px-3">
          {items.map(({ to, key, icon: Icon }) => (
            <li key={to}>
              <Link to={to} activeOptions={{ exact: to === '/hris' }}
                className="flex items-center gap-3 whitespace-nowrap rounded-control px-3 py-2.5 text-sm font-semibold text-text-secondary hover:bg-accent hover:text-foreground"
                activeProps={{ className: cn('bg-secondary text-foreground') }}>
                <Icon aria-hidden className="size-5" />
                {t(`hris.nav.${key}`)}
              </Link>
            </li>
          ))}
          <li className="lg:mt-4 lg:border-t lg:border-border-subtle lg:pt-4">
            <Link to="/" className="flex items-center gap-3 whitespace-nowrap rounded-control px-3 py-2.5 text-sm font-semibold text-muted-foreground hover:bg-accent">
              <ArrowLeftIcon aria-hidden className="size-5" />
              {t('admin.nav.back')}
            </Link>
          </li>
        </ul>
      </nav>
      <main className="mx-auto w-full max-w-[1440px] flex-1 px-4 py-6 lg:px-12">
        <Outlet />
      </main>
    </div>
  );
}
