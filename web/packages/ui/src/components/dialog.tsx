import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '../lib/cn';

export interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  /** 'sheet' slides in from the right (editors); 'dialog' is centred (confirmations). */
  variant?: 'dialog' | 'sheet';
  className?: string;
}

export function Dialog({ open, onOpenChange, title, description, children, footer, variant = 'dialog', className }: DialogProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-[60] bg-[var(--sl-color-bg-scrim)]/60 data-[state=open]:animate-in" />
        <DialogPrimitive.Content
          className={cn(
            'fixed z-[60] flex max-h-dvh flex-col bg-card text-card-foreground shadow-xl focus:outline-none',
            variant === 'dialog' &&
              'left-1/2 top-1/2 w-[calc(100%-32px)] max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-sheet',
            variant === 'sheet' && 'inset-y-0 right-0 w-full max-w-2xl',
            className,
          )}
        >
          <header className="flex items-start gap-3 border-b border-border-subtle px-5 py-4">
            <div className="min-w-0 flex-1">
              <DialogPrimitive.Title className="text-lg font-bold">{title}</DialogPrimitive.Title>
              {description ? (
                <DialogPrimitive.Description className="text-sm text-muted-foreground">{description}</DialogPrimitive.Description>
              ) : (
                <DialogPrimitive.Description className="sr-only">{title}</DialogPrimitive.Description>
              )}
            </div>
            <DialogPrimitive.Close className="rounded-control p-2 text-muted-foreground hover:bg-accent" aria-label="Tutup">
              <X aria-hidden className="size-5" />
            </DialogPrimitive.Close>
          </header>
          <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
          {footer && <footer className="flex justify-end gap-2 border-t border-border-subtle px-5 py-3">{footer}</footer>}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
