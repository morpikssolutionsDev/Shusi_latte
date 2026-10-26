import lockupAnimated from '../brand/sushi-latte-lockup-animated.svg?url';
import lockup from '../brand/sushi-latte-lockup.svg?url';
import logoAnimated from '../brand/sushi-latte-logo-animated.svg?url';
import logo from '../brand/sushi-latte-logo.svg?url';

export interface LogoProps {
  variant?: 'badge' | 'lockup';
  animated?: boolean;
  className?: string;
}

/** Brand logo (docs/brand). Animated variants replay their intro when re-mounted. */
export function Logo({ variant = 'badge', animated = false, className }: LogoProps) {
  const src = variant === 'lockup' ? (animated ? lockupAnimated : lockup) : animated ? logoAnimated : logo;
  return <img src={src} alt="Sushi Latte" className={className} draggable={false} />;
}
