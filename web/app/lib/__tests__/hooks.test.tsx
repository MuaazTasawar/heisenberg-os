import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDebouncedValue } from "../hooks";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useDebouncedValue", () => {
  it("starts with the value it is given", () => {
    const { result } = renderHook(() => useDebouncedValue(5, 300));
    expect(result.current).toBe(5);
  });

  it("only follows a value once it has stopped changing", () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value, 300), { initialProps: { value: 1 } });

    rerender({ value: 2 });
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe(1);

    rerender({ value: 3 });
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe(1); // the second change restarted the wait

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(3);
  });

  it("never shows the values that were skipped", () => {
    const seen: number[] = [];
    const { rerender } = renderHook(
      ({ value }) => {
        const debounced = useDebouncedValue(value, 100);
        seen.push(debounced);
        return debounced;
      },
      { initialProps: { value: 0 } },
    );
    for (const value of [1, 2, 3]) {
      rerender({ value });
      act(() => {
        vi.advanceTimersByTime(50);
      });
    }
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(new Set(seen)).toEqual(new Set([0, 3]));
  });

  it("does not update after it is unmounted", () => {
    const { rerender, unmount } = renderHook(({ value }) => useDebouncedValue(value, 100), { initialProps: { value: 1 } });
    rerender({ value: 2 });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
