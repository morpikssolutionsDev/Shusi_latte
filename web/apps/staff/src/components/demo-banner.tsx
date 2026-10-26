import { useQuery } from '@tanstack/react-query';
import { FlaskConical } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { healthQuery } from '../session';

/** DEMO-AND-SEED §2: shown on every screen while Demo mode is on. */
export function DemoBanner() {
  const { t } = useTranslation();
  const { data } = useQuery(healthQuery);
  if (!data?.demo) return null;
  return (
    <div className="flex items-center justify-center gap-2 bg-nori px-4 py-1.5 text-xs font-semibold text-rice">
      <FlaskConical aria-hidden className="size-3.5" />
      {t('demo.banner')}
    </div>
  );
}
