import { formatMoney } from "./money";

describe("formatMoney", () => {
  it("formats GEL tetri without floating point arithmetic", () => {
    expect(formatMoney({ currency: "GEL", amount: 12345 })).toBe("₾123.45");
  });

  it("formats USD cents independently", () => {
    expect(formatMoney({ currency: "USD", amount: 1250 })).toBe("$12.50");
  });
});
