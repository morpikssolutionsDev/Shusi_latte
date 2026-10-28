import { api, unwrap } from '@sushi-latte/api-client';
import { Button, Dialog, cn } from '@sushi-latte/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { LogOut } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Ends a screen-mode session (Kitchen, Bar, HRIS tablet) and returns the device to sign-in
 * (TR-AUTH-08). No PIN: the owner chose a plain exit; the confirmation only catches stray taps.
 */
export function ExitScreenButton({ className }: { className?: string }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const exit = useMutation({
    mutationFn: async () => unwrap(await api.POST('/auth/logout')),
    onSettled: () => {
      queryClient.setQueryData(['me'], null);
      void navigate({ to: '/login' });
    },
  });
  return (
    <>
      <Button variant="ghost" size="md" className={cn('text-muted-foreground', className)} onClick={() => setOpen(true)} data-testid="exit-screen">
        <LogOut aria-hidden />
        {t('home.exitScreen')}
      </Button>
      <Dialog open={open} onOpenChange={setOpen} title={t('home.exitScreenTitle')}
        footer={<>
          <Button variant="secondary" onClick={() => setOpen(false)}>{t('admin.common.cancel')}</Button>
          <Button disabled={exit.isPending} onClick={() => exit.mutate()} data-testid="exit-screen-confirm">{t('home.exitScreen')}</Button>
        </>}>
        <p className="text-muted-foreground">{t('home.exitScreenHint')}</p>
      </Dialog>
    </>
  );
}
