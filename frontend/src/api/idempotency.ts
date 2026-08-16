export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

export function retainIdempotencyKey(key: string | undefined): string {
  return key ?? newIdempotencyKey();
}
