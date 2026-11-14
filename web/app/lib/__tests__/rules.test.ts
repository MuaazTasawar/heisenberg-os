import { describe, expect, it } from "vitest";
import { aFront, aRules, anEmpire } from "../../test-utils";
import { activeFronts, describeRules, territoryStatus } from "../rules";

describe("describeRules", () => {
  it("states every rule the server sends", () => {
    const lines = describeRules(aRules());
    expect(lines.map((line) => line.id)).toEqual(["cooking", "price", "heat", "laundering", "flags", "territory", "fronts"]);
    const text = lines.map((line) => line.text).join("\n");
    expect(text).toContain("from 1 to 10");
    expect(text).toContain("about 8.33");
    expect(text).toContain("$1,500");
    expect(text).toContain("2.5 heat");
    expect(text).toContain("6 per hour");
    expect(text).toContain("75% of what you send");
    expect(text).toContain("$50,000");
    expect(text).toContain("5% to your yield, up to 10 territories");
    expect(text).toContain("up to 5 front businesses");
  });

  it("follows the server, not what the numbers happen to be today", () => {
    const text = describeRules(
      aRules({ min_effort: 2, max_effort: 12, optimal_effort: 9.5, dirty_per_kg: 2000, launder_rate: 0.6, large_transfer: 20000, territory_bonus: 0.1, max_territories: 7, max_active_fronts: 3, heat_decay_per_hour: 4 }),
    ).map((line) => line.text).join("\n");
    for (const expected of ["from 2 to 12", "about 9.5", "$2,000", "60% of what you send", "$20,000", "10% to your yield, up to 7 territories", "up to 3 front businesses", "4 per hour"]) {
      expect(text, expected).toContain(expected);
    }
    for (const stale of ["75%", "$50,000", "$1,500", "8.33"]) expect(text, stale).not.toContain(stale);
  });
});

describe("territoryStatus", () => {
  it("is at the maximum when the server has no next price", () => {
    expect(territoryStatus(aRules({ next_territory_cost: null }), { clean_money: 1e9 })).toEqual({ atMax: true, cost: null, shortfall: 0, affordable: false });
  });

  it("tells how much is missing, and is affordable at exactly the price", () => {
    const rules = aRules({ next_territory_cost: 100000 });
    expect(territoryStatus(rules, { clean_money: 45000 })).toEqual({ atMax: false, cost: 100000, shortfall: 55000, affordable: false });
    expect(territoryStatus(rules, { clean_money: 100000 })).toMatchObject({ shortfall: 0, affordable: true });
    expect(territoryStatus(rules, { clean_money: 250000 })).toMatchObject({ shortfall: 0, affordable: true });
  });

  it("does not leave floating point crumbs in the shortfall", () => {
    expect(territoryStatus(aRules({ next_territory_cost: 100 }), { clean_money: 99.9 }).shortfall).toBe(0.1);
  });
});

describe("activeFronts", () => {
  it("counts only the open ones", () => {
    expect(activeFronts(anEmpire({ front_businesses: [aFront({ id: 1 }), aFront({ id: 2, is_active: false }), aFront({ id: 3 })] }))).toBe(2);
    expect(activeFronts(anEmpire())).toBe(0);
  });

  it("copes with a missing list", () => {
    expect(activeFronts({ front_businesses: undefined as never })).toBe(0);
  });
});
