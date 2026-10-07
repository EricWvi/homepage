/**
 * A uniformly random integer in [0, n) from the platform's cryptographic
 * generator rather than Math.random. Rejection sampling avoids the bias a
 * plain modulo would add.
 */
export function randomIndex(n: number): number {
  if (n <= 1) return 0;
  const limit = Math.floor(0x1_0000_0000 / n) * n;
  const buf = new Uint32Array(1);
  for (;;) {
    crypto.getRandomValues(buf);
    if (buf[0]! < limit) return buf[0]! % n;
  }
}
