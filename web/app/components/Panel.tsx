import { useId, type ReactNode } from "react";

interface PanelProps {
  title?: string;
  subtitle?: ReactNode;
  /** Buttons or links shown at the right of the heading. */
  actions?: ReactNode;
  className?: string;
  children?: ReactNode;
}

/** A titled box; the title names the region for screen readers. */
export default function Panel({ title, subtitle, actions, className, children }: PanelProps) {
  const titleId = useId();
  return (
    <section className={className ? `panel ${className}` : "panel"} aria-labelledby={title ? titleId : undefined}>
      {(title || actions) && (
        <header className="panel__head">
          <div>
            {title && (
              <h2 className="panel__title" id={titleId}>
                {title}
              </h2>
            )}
            {subtitle && <p className="panel__subtitle">{subtitle}</p>}
          </div>
          {actions && <div className="panel__actions">{actions}</div>}
        </header>
      )}
      {children}
    </section>
  );
}
