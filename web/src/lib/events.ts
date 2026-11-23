import type { User } from "./types";

// Components stay decoupled from the router: pages and the header report what happened and the app shell reacts.
export type SpotrebaEvents = {
  /** Go to another route, optionally with a flash message. Both messages are reset when omitted. */
  "spotreba:navigate": { path: string; notice?: string; error?: string };
  /** Change the flash messages without navigating. Omitted messages are left untouched; "" clears one. */
  "spotreba:flash": { notice?: string; error?: string };
  /** A session was started. Without a path the user returns to the page they originally asked for. */
  "spotreba:signed-in": { user: User; path?: string; notice?: string };
  "spotreba:signed-out": void;
};

declare global {
  interface HTMLElementEventMap {
    "spotreba:navigate": CustomEvent<SpotrebaEvents["spotreba:navigate"]>;
    "spotreba:flash": CustomEvent<SpotrebaEvents["spotreba:flash"]>;
    "spotreba:signed-in": CustomEvent<SpotrebaEvents["spotreba:signed-in"]>;
    "spotreba:signed-out": CustomEvent<void>;
  }
}

export type Redirect = SpotrebaEvents["spotreba:navigate"];

export function emit<K extends keyof SpotrebaEvents>(
  source: EventTarget,
  type: K,
  ...detail: SpotrebaEvents[K] extends void ? [] : [SpotrebaEvents[K]]
) {
  source.dispatchEvent(new CustomEvent(type, { detail: detail[0], bubbles: true, composed: true }));
}
