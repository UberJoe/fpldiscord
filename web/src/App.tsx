// App shell: single column, hamburger nav, lands on Standings. Client-side
// routing is a hand-rolled history-API hook (src/router.ts); the Go server's
// SPA fallback makes deep links work.

import { useState } from "react";
import { usePath, matchManager, navigate } from "./router";
import { Standings } from "./views/Standings";
import { Manager } from "./views/Manager";
import { Waivers } from "./views/Waivers";
import { Bet } from "./views/Bet";

interface NavItem {
  path: string;
  label: string;
}

const NAV: NavItem[] = [
  { path: "/", label: "Standings" },
  { path: "/waivers", label: "Waiver History" },
  { path: "/bet", label: "Bet" },
];

function Body({ path }: { path: string }) {
  const managerId = matchManager(path);
  // key by id so navigating between two manager routes remounts the view and
  // its poll starts fresh on the new endpoint.
  if (managerId !== null) return <Manager key={managerId} entryId={managerId} />;
  switch (path) {
    case "/":
      return <Standings />;
    case "/waivers":
      return <Waivers />;
    case "/bet":
      return <Bet />;
    default:
      return <p className="status">Not found.</p>;
  }
}

export function App() {
  const path = usePath();
  const [menuOpen, setMenuOpen] = useState(false);

  const go = (to: string) => {
    setMenuOpen(false);
    navigate(to);
  };

  const activeTop = matchManager(path) !== null ? "/" : path;

  return (
    <div className="app">
      <header className="topbar">
        <button
          className="hamburger"
          aria-label="Menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((v) => !v)}
        >
          ☰
        </button>
        <span className="title">FPL Draft</span>
      </header>

      {menuOpen && (
        <nav className="drawer">
          {NAV.map((item) => (
            <button
              key={item.path}
              className={item.path === activeTop ? "active" : ""}
              onClick={() => go(item.path)}
            >
              {item.label}
            </button>
          ))}
        </nav>
      )}

      <main className="content">
        <Body path={path} />
      </main>
    </div>
  );
}
