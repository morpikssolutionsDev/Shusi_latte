import '@sushi-latte/ui/styles.css';
import './menu.css';
import './i18n';

import { Button, mountApp } from '@sushi-latte/ui';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Outlet, RouterProvider, createRootRoute, createRoute, createRouter, lazyRouteComponent, useNavigate } from '@tanstack/react-router';
import { QrCodeIcon, SmartphoneIcon } from 'lucide-react';
import { AnimatePresence } from 'motion/react';
import { StrictMode, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Toaster } from 'sonner';
import { healthQuery, meQuery, menuQuery, useGuestRealtime, type CustomerView, type Menu } from './api';
import { loadCart } from './cart';

import { CenteredLoader, JoinFlow, Notice, Splash } from './join';
import { MenuScreen } from './menu';
import { OrdersScreen } from './orders';
import { clearSession, resume, setEnded, useEnded, useSession } from './session';

/**
 * Everything behind a Table QR: resume or join (OP-01), then load the Customer's view and
 * follow it live. Ended sessions show their plain notice (M12).
 */
function GuestGate({ qr, fresh, children }: { qr: string; fresh: boolean; children: (me: CustomerView, menu: Menu, online: boolean) => ReactNode }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  useState(() => resume(qr, fresh));
  const session = useSession();
  const ended = useEnded();
  const [splash, setSplash] = useState(false);
  const me = useQuery({ ...meQuery(session?.customerId ?? ''), enabled: !!session && !ended });
  const menu = useQuery(menuQuery);
  const online = useGuestRealtime(ended ? null : session, me.data?.bill.id);
  if (session) loadCart(session.customerId);

  const restart = () => {
    clearSession(qr);
    setEnded(null);
    void navigate({ to: '/t/$qr', params: { qr }, search: { fresh: true } });
  };
  if (ended === 'continued_elsewhere') {
    return <Notice icon={<SmartphoneIcon className="size-8" aria-hidden />} title={t('ended.elsewhereTitle')} body={t('ended.elsewhereBody')} action={<Button onClick={restart}>{t('ended.scanAgain')}</Button>} />;
  }
  if (ended) {
    return <Notice title={t('ended.closedTitle')} body={t('ended.closedBody')} action={<Button onClick={restart}>{t('ended.scanAgain')}</Button>} />;
  }
  if (!session || session.qr !== qr) return <JoinFlow qr={qr} onJoined={(first) => setSplash(first)} />;
  return (
    <>
      {me.data && menu.data ? children(me.data, menu.data, online) : <CenteredLoader />}
      <AnimatePresence>{splash && <Splash onDone={() => setSplash(false)} />}</AnimatePresence>
    </>
  );
}

function Landing() {
  const { t } = useTranslation();
  const health = useQuery(healthQuery);
  return (
    <Notice
      icon={<QrCodeIcon className="size-8" aria-hidden />}
      title={t('scan.title')}
      body={t('scan.body')}
      action={health.data?.demo && <a href="/demo" className="text-sm font-semibold text-text-brand underline">{t('demo.launcher')}</a>}
    />
  );
}

const root = createRootRoute({
  component: () => (
    <>
      <Outlet />
      <Toaster position="top-center" richColors closeButton={false} />
    </>
  ),
});
const search = (s: Record<string, unknown>): { fresh?: boolean } => (s.fresh === true || s.fresh === '1' || s.fresh === 1 ? { fresh: true } : {});
const landing = createRoute({ getParentRoute: () => root, path: '/', component: Landing });
const demo = createRoute({ getParentRoute: () => root, path: '/demo', component: lazyRouteComponent(() => import('./demo'), 'DemoLauncher') });
const table = createRoute({
  getParentRoute: () => root,
  path: '/t/$qr',
  validateSearch: search,
  component: function TableRoute() {
    const { qr } = table.useParams();
    const { fresh } = table.useSearch();
    return <GuestGate qr={qr} fresh={!!fresh}>{(me, menu, online) => <MenuScreen qr={qr} me={me} menu={menu} online={online} />}</GuestGate>;
  },
});
const orders = createRoute({
  getParentRoute: () => root,
  path: '/t/$qr/orders',
  component: function OrdersRoute() {
    const { qr } = orders.useParams();
    return <GuestGate qr={qr} fresh={false}>{(me) => <OrdersScreen qr={qr} me={me} />}</GuestGate>;
  },
});

const router = createRouter({ routeTree: root.addChildren([landing, demo, table, orders]), scrollRestoration: true });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: true } } });

mountApp(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
