import { useCallback } from "react";
import { useSearchParams } from "react-router-dom";

/**
 * Sync a string value with a URL search param (like useState, but shareable).
 * Omits the param when the value equals the default so URLs stay clean.
 */
export function useQueryState(
  key: string,
  defaultValue: string,
): [string, (value: string) => void] {
  const [searchParams, setSearchParams] = useSearchParams();
  const value = searchParams.get(key) ?? defaultValue;

  const setValue = useCallback(
    (next: string) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (!next || next === defaultValue) params.delete(key);
          else params.set(key, next);
          return params;
        },
        { replace: true },
      );
    },
    [defaultValue, key, setSearchParams],
  );

  return [value, setValue];
}
