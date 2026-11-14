import { formatMoney, formatNumber, formatPercent } from "./format";
import type { Empire, Rules } from "./types";

export interface RuleLine {
  id: string;
  text: string;
}

/**
 * The rules of the game in words, built from what the server says (`rules` in /game/state), so
 * the page can never disagree with the backend about a number.
 */
export function describeRules(rules: Rules): RuleLine[] {
  return [
    {
      id: "cooking",
      text: `Cooking effort runs from ${formatNumber(rules.min_effort, 1)} to ${formatNumber(rules.max_effort, 1)}; the sweet spot is about ${formatNumber(rules.optimal_effort, 2)}.`,
    },
    { id: "price", text: `Every kilogram sells for ${formatMoney(rules.dirty_per_kg)} in dirty money.` },
    {
      id: "heat",
      text: `Each point of effort adds ${formatNumber(rules.heat_per_effort, 1)} heat; heat cools by ${formatNumber(rules.heat_decay_per_hour, 1)} per hour.`,
    },
    {
      id: "laundering",
      text: `Laundering turns ${formatPercent(rules.launder_rate * 100)} of what you send into clean money (at least ${formatMoney(rules.launder_min)} per transfer).`,
    },
    { id: "flags", text: `A single transfer above ${formatMoney(rules.large_transfer)} is flagged as evidence.` },
    {
      id: "territory",
      text: `Each territory adds ${formatPercent(rules.territory_bonus * 100)} to your yield, up to ${rules.max_territories} territories.`,
    },
    { id: "fronts", text: `You can run up to ${rules.max_active_fronts} front businesses at once.` },
  ];
}

export interface TerritoryStatus {
  /** The player holds the maximum number of territories. */
  atMax: boolean;
  cost: number | null;
  /** How much more clean money is needed; 0 when the player can afford it. */
  shortfall: number;
  affordable: boolean;
}

export function territoryStatus(rules: Rules, empire: Pick<Empire, "clean_money">): TerritoryStatus {
  const cost = rules.next_territory_cost;
  if (cost === null) return { atMax: true, cost: null, shortfall: 0, affordable: false };
  const shortfall = Math.max(0, Math.round((cost - empire.clean_money) * 100) / 100);
  return { atMax: false, cost, shortfall, affordable: shortfall === 0 };
}

/** Fronts that are still open. */
export function activeFronts(empire: Pick<Empire, "front_businesses">): number {
  return (empire.front_businesses ?? []).filter((front) => front.is_active).length;
}
