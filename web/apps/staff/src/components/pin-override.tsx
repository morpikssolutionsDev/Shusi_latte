// PinOverrideDialog (COMPONENTS.md §3, TR-AUTH-06): the approving Outlet Admin or Management
// enters their Crew ID and PIN on this device; the API returns a one-time token for one action.
import { api, ApiError, unwrap, type OverrideAction } from '@sushi-latte/api-client';
import { Dialog, Field, Input, PinPad } from '@sushi-latte/ui';
import { useMutation } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

export function PinOverrideDialog({ action, open, onApproved, onClose }: {
  action: OverrideAction;
  open: boolean;
  onApproved: (token: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [crewId, setCrewId] = useState('');
  const [pin, setPin] = useState('');
  const [shake, setShake] = useState(0);
  useEffect(() => {
    if (open) {
      setCrewId('');
      setPin('');
    }
  }, [open]);
  const approve = useMutation({
    mutationFn: async (p: string) => unwrap(await api.POST('/auth/override', { body: { crewId: crewId.trim().toUpperCase(), pin: p, action } })),
    onSuccess: (res) => onApproved(res.token),
    onError: () => {
      setPin('');
      setShake((n) => n + 1);
    },
  });
  const error = approve.error instanceof ApiError ? (approve.error.detail ?? approve.error.message) : approve.error ? t('waiter.errors.generic') : '';
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()} title={t('waiter.pin.title')} description={t('waiter.pin.approverHint')}>
      <div className="flex flex-col items-center gap-4">
        <Field label={t('waiter.pin.approver')} htmlFor="override-crew" className="w-full">
          <Input id="override-crew" value={crewId} autoComplete="off" placeholder="A001" className="font-mono uppercase" onChange={(e) => setCrewId(e.target.value)} />
        </Field>
        <PinPad
          key={shake}
          value={pin}
          onChange={setPin}
          error={shake > 0}
          disabled={!crewId.trim() || approve.isPending}
          onComplete={(p) => approve.mutate(p)}
          label={t('waiter.pin.pin')}
        />
        {error && <p className="text-sm font-semibold text-[var(--sl-color-feedback-danger-fg)]" role="alert">{error}</p>}
      </div>
    </Dialog>
  );
}
