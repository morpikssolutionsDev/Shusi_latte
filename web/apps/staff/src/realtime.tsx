import { connectRealtime, type RealtimeEvent } from '@sushi-latte/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';

type Status = 'connecting' | 'open' | 'closed';

/**
 * Subscribes to realtime topics while mounted. Events are hints to refetch (TR-RT-03);
 * after a reconnect every active query is refetched so missed events do not matter (TR-RT-04).
 */
export function useRealtime(topics: string[], onEvent?: (e: RealtimeEvent) => void, enabled = true) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<Status>('connecting');
  const handler = useRef(onEvent);
  handler.current = onEvent;
  const key = topics.join('|');

  useEffect(() => {
    if (!enabled) return;
    const conn = connectRealtime({
      topics: key.split('|').filter(Boolean),
      onEvent: (e) => handler.current?.(e),
      onReconnect: () => void queryClient.invalidateQueries(),
      onStatus: setStatus,
    });
    return () => conn.close();
  }, [key, enabled, queryClient]);

  return status;
}
