// Tiny history-API router — no dependency. The Go server's SPA fallback serves
// index.html for any unknown path, so deep links like /manager/39880 load the
// app and this hook reads the path.

import { useEffect, useState } from "react";

export function navigate(to: string): void {
  if (to === window.location.pathname) return;
  window.history.pushState(null, "", to);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function usePath(): string {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  return path;
}

/** matchManager returns the entryId for a /manager/:id path, else null. */
export function matchManager(path: string): number | null {
  const m = /^\/manager\/(\d+)$/.exec(path);
  return m ? Number(m[1]) : null;
}
