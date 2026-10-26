/**
 * WebSocket client for /ws (TRD §7). Events carry ids only; consumers refetch through
 * TanStack Query. On every (re)connect `onReconnect` fires so callers refetch all active
 * queries, which makes missed events harmless (TR-RT-03, TR-RT-04).
 */
export interface RealtimeEvent {
  topic: string;
  type: string;
  id?: string;
  data?: unknown;
}

export interface RealtimeConnection {
  close(): void;
  readonly status: () => 'connecting' | 'open' | 'closed';
}

interface Options {
  topics: string[];
  onEvent: (event: RealtimeEvent) => void;
  onReconnect?: () => void;
  onStatus?: (status: 'connecting' | 'open' | 'closed') => void;
  url?: string;
}

const MAX_BACKOFF_MS = 10_000;

export function connectRealtime({ topics, onEvent, onReconnect, onStatus, url }: Options): RealtimeConnection {
  let socket: WebSocket | undefined;
  let attempt = 0;
  let stopped = false;
  let state: 'connecting' | 'open' | 'closed' = 'connecting';
  let timer: ReturnType<typeof setTimeout> | undefined;

  const setState = (s: typeof state) => {
    state = s;
    onStatus?.(s);
  };

  const open = () => {
    setState('connecting');
    const target = url ?? `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`;
    socket = new WebSocket(target);
    socket.onopen = () => {
      const isReconnect = attempt > 0;
      attempt = 0;
      setState('open');
      socket?.send(JSON.stringify({ op: 'subscribe', topics }));
      if (isReconnect) onReconnect?.();
    };
    socket.onmessage = (msg) => {
      const data = JSON.parse(String(msg.data)) as { type: string; event?: RealtimeEvent };
      if (data.type === 'event' && data.event) onEvent(data.event);
    };
    socket.onclose = () => {
      setState('closed');
      if (stopped) return;
      attempt += 1;
      const delay = Math.min(MAX_BACKOFF_MS, 500 * 2 ** (attempt - 1)) * (0.75 + Math.random() * 0.5);
      timer = setTimeout(open, delay);
    };
  };

  open();
  return {
    close() {
      stopped = true;
      clearTimeout(timer);
      socket?.close();
    },
    status: () => state,
  };
}
