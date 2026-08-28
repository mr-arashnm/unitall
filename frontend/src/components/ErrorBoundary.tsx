// React error boundary: catches render-time exceptions so one bad
// screen doesn't blank the entire app. Without this, a single throw
// anywhere in a screen wipes the <div id="root"> and the user sees a
// blank page with no way to recover except reloading.
import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}
interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: { componentStack?: string }) {
    // Surface to console for the dev/devtools session, and a structured
    // log so production debugging isn't blind.
    // eslint-disable-next-line no-console
    console.error("[ErrorBoundary]", error, info);
  }

  render() {
    if (this.state.error) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="view" style={{ padding: 24 }}>
          <div className="card stack" style={{ borderColor: "var(--c-danger)" }}>
            <h2 style={{ margin: 0 }}>Something went wrong</h2>
            <p className="secondary small" style={{ margin: 0 }}>
              {this.state.error.message}
            </p>
            <button
              className="btn btn-secondary"
              onClick={() => this.setState({ error: null })}
            >
              Try again
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
