import { Fragment, type ReactNode } from "react";

interface AuthShellProps {
  kicker: string;
  /** The heading, one line per entry. */
  title: string[];
  children: ReactNode;
  footer: ReactNode;
}

/** The frame of the sign-in and registration pages. */
export default function AuthShell({ kicker, title, children, footer }: AuthShellProps) {
  return (
    <main className="auth">
      <div className="auth__card">
        <header className="auth__head">
          <p className="auth__kicker">{kicker}</p>
          <h1 className="auth__title">
            {title.map((line, index) => (
              <Fragment key={line}>
                {index > 0 && " "}
                <span>{line}</span>
              </Fragment>
            ))}
          </h1>
        </header>
        {children}
        <p className="auth__footer">{footer}</p>
      </div>
    </main>
  );
}
