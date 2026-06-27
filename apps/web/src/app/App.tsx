import { Button } from "../components/ui/button";
import { zhCN } from "../i18n/zh-CN";
import "../styles/globals.css";

export function App() {
  const text = zhCN.foundation;

  return (
    <main aria-label={text.mainLabel} className="min-h-screen bg-bg px-8 py-10 text-fg">
      <section className="mx-auto grid max-w-5xl gap-6 border border-white/15 bg-white/[0.04] p-8">
        <p className="text-xs font-semibold uppercase tracking-[0.08em] text-muted">{text.eyebrow}</p>
        <div className="grid gap-3">
          <h1 className="text-3xl font-semibold tracking-normal">{text.title}</h1>
          <p className="max-w-2xl text-sm leading-7 text-muted">{text.summary}</p>
        </div>
        <div>
          <Button variant="primary">{text.primaryAction}</Button>
        </div>
      </section>
    </main>
  );
}
