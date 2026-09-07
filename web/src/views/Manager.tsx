// View — manager drill-down (route /manager/:id). The live gameweek squad with
// auto-subs applied lands in ticket 06; ticket 05 wires the route and the back
// affordance so deep links already resolve.

import { navigate } from "../router";

export function Manager({ entryId }: { entryId: number }) {
  return (
    <div className="manager">
      <button className="back" onClick={() => navigate("/")}>
        ← Standings
      </button>
      <p className="status">
        Manager {entryId}'s gameweek squad arrives soon.
      </p>
    </div>
  );
}
