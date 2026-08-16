export interface APIError {
  status: number;
  code: string;
  message: string;
}

export function errorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error) {
    return String((error as { message: unknown }).message);
  }
  return "The Fees service is unavailable. Please retry.";
}
