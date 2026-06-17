import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";

/**
 * Shared button for the VER-240 theme. Renders an <a>/Link or <button>
 * with the correct hover variant. Anchor buttons keep their label color
 * on hover (no global link-blue bleed) — see globals.css `.btn`.
 */
export type ButtonVariant =
  | "primary" // yellow — brand / primary action
  | "secondary" // blue — links / secondary / trust action
  | "ghost" // neutral outline on light surfaces
  | "ghost-invert" // outline on dark surfaces (hero) — translucent hover
  | "dark";

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: "btn-primary",
  secondary: "btn-secondary",
  ghost: "btn-ghost",
  "ghost-invert": "btn-ghost-invert",
  dark: "btn-dark",
};

type CommonProps = {
  variant?: ButtonVariant;
  size?: "md" | "sm";
  block?: boolean;
  className?: string;
  children: ReactNode;
};

function classes({ variant = "primary", size = "md", block, className }: CommonProps) {
  return [
    "btn",
    VARIANT_CLASS[variant],
    size === "sm" && "btn-sm",
    block && "btn-block",
    className,
  ]
    .filter(Boolean)
    .join(" ");
}

type ButtonAsButton = CommonProps &
  Omit<ComponentProps<"button">, "className" | "children"> & { href?: undefined };

type ButtonAsLink = CommonProps &
  Omit<ComponentProps<typeof Link>, "className" | "children" | "href"> & { href: string };

export default function Button(props: ButtonAsButton | ButtonAsLink) {
  const { variant, size, block, className, children, ...rest } = props;
  const cls = classes({ variant, size, block, className, children });

  if ("href" in props && props.href !== undefined) {
    const { href, ...linkRest } = rest as ButtonAsLink;
    return (
      <Link href={href} className={cls} {...linkRest}>
        {children}
      </Link>
    );
  }

  return (
    <button className={cls} {...(rest as ButtonAsButton)}>
      {children}
    </button>
  );
}
