import * as LabelPrimitive from '@radix-ui/react-label';
import type { ComponentProps, InputHTMLAttributes } from 'react';
import { cn } from '../lib/cn';

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        'h-11 w-full rounded-control border border-input bg-card px-4 text-base text-foreground placeholder:text-muted-foreground',
        'focus-visible:border-ring disabled:opacity-40',
        className,
      )}
      {...props}
    />
  );
}

export function Label({ className, ...props }: ComponentProps<typeof LabelPrimitive.Root>) {
  return <LabelPrimitive.Root className={cn('text-sm font-semibold text-foreground', className)} {...props} />;
}
