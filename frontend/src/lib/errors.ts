// errorMessage extracts a human-readable message from an unknown thrown value,
// falling back to the supplied default when none is available.
export function errorMessage(e: unknown, fallback = "Something went wrong."): string {
  if (e instanceof Error && e.message) return e.message;
  return fallback;
}
