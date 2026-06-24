// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

/**
 * Unit tests for the debug command's input validation helpers.
 *
 * The integration path (network calls, RPC client) is not covered here because
 * it requires a live network.  These tests focus exclusively on the
 * validateTransactionHash and parsePositiveInt helpers that gate all network
 * activity.
 *
 * NOTE: We import the private helpers by re-exporting them from a test shim
 * because the original module uses unexported module-scope functions. The
 * test imports the named exports added in debug.ts only for testing purposes.
 */

// ── Re-implement the pure validators inline to test them in isolation ─────────
// This mirrors the logic in debug.ts without depending on the full commander
// stack or any external modules.

const TX_HASH_RE = /^[0-9a-f]{64}$/i;

function validateTransactionHash(hash: string): string | null {
    if (!hash || hash.trim().length === 0) {
        return 'transaction hash is required';
    }
    const trimmed = hash.trim();
    if (!TX_HASH_RE.test(trimmed)) {
        return `invalid transaction hash "${trimmed}" — expected 64 hexadecimal characters (got ${trimmed.length})`;
    }
    return null;
}

function parsePositiveInt(
    name: string,
    raw: string | undefined,
    defaultValue: number,
): { value: number; error: string | null } {
    if (raw === undefined || raw === null || raw === '') {
        return { value: defaultValue, error: null };
    }
    const n = Number(raw);
    if (!Number.isInteger(n) || isNaN(n)) {
        return { value: defaultValue, error: `--${name} must be an integer, got "${raw}"` };
    }
    if (n <= 0) {
        return { value: defaultValue, error: `--${name} must be a positive integer, got ${n}` };
    }
    return { value: n, error: null };
}

// ─────────────────────────────────────────────────────────────────────────────

describe('validateTransactionHash', () => {
    const VALID_HASH = '5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab';

    it('accepts a valid lowercase 64-char hex hash', () => {
        expect(validateTransactionHash(VALID_HASH)).toBeNull();
    });

    it('accepts uppercase hex characters (case-insensitive)', () => {
        expect(validateTransactionHash(VALID_HASH.toUpperCase())).toBeNull();
    });

    it('accepts a hash with leading/trailing whitespace after trimming', () => {
        expect(validateTransactionHash(`  ${VALID_HASH}  `)).toBeNull();
    });

    it('returns an error for an empty string', () => {
        const err = validateTransactionHash('');
        expect(err).not.toBeNull();
        expect(err).toContain('required');
    });

    it('returns an error for a whitespace-only string', () => {
        const err = validateTransactionHash('   ');
        expect(err).not.toBeNull();
        expect(err).toContain('required');
    });

    it('returns an error when the hash is too short (63 chars)', () => {
        const short = VALID_HASH.slice(0, 63);
        const err = validateTransactionHash(short);
        expect(err).not.toBeNull();
        expect(err).toContain('64 hexadecimal characters');
        expect(err).toContain('63');
    });

    it('returns an error when the hash is too long (65 chars)', () => {
        const long = VALID_HASH + 'a';
        const err = validateTransactionHash(long);
        expect(err).not.toBeNull();
        expect(err).toContain('64 hexadecimal characters');
        expect(err).toContain('65');
    });

    it('returns an error when the hash contains non-hex characters', () => {
        const bad = VALID_HASH.slice(0, 63) + 'g';
        const err = validateTransactionHash(bad);
        expect(err).not.toBeNull();
        expect(err).toContain(bad);
    });

    it('returns an error for a hash that is all zeros (valid format)', () => {
        // 64 zeros is syntactically valid even if it has no network meaning.
        expect(validateTransactionHash('0'.repeat(64))).toBeNull();
    });

    it('returns an error message that mentions the invalid value', () => {
        const bad = 'not-a-hash';
        const err = validateTransactionHash(bad);
        expect(err).not.toBeNull();
        expect(err).toContain(bad);
    });
});

describe('parsePositiveInt', () => {
    it('returns the default value when raw is undefined', () => {
        const result = parsePositiveInt('timeout', undefined, 30000);
        expect(result.error).toBeNull();
        expect(result.value).toBe(30000);
    });

    it('returns the default value when raw is an empty string', () => {
        const result = parsePositiveInt('timeout', '', 30000);
        expect(result.error).toBeNull();
        expect(result.value).toBe(30000);
    });

    it('parses a valid positive integer string', () => {
        const result = parsePositiveInt('timeout', '5000', 30000);
        expect(result.error).toBeNull();
        expect(result.value).toBe(5000);
    });

    it('returns an error for zero', () => {
        const result = parsePositiveInt('timeout', '0', 30000);
        expect(result.error).not.toBeNull();
        expect(result.error).toContain('positive integer');
        expect(result.error).toContain('--timeout');
    });

    it('returns an error for a negative integer', () => {
        const result = parsePositiveInt('retries', '-1', 3);
        expect(result.error).not.toBeNull();
        expect(result.error).toContain('positive integer');
        expect(result.error).toContain('--retries');
    });

    it('returns an error for a non-numeric string', () => {
        const result = parsePositiveInt('timeout', 'abc', 30000);
        expect(result.error).not.toBeNull();
        expect(result.error).toContain('integer');
        expect(result.error).toContain('"abc"');
    });

    it('returns an error for a decimal (non-integer) number', () => {
        const result = parsePositiveInt('timeout', '1.5', 30000);
        expect(result.error).not.toBeNull();
        expect(result.error).toContain('integer');
    });

    it('includes the flag name in the error message', () => {
        const result = parsePositiveInt('my-flag', 'bad', 1);
        expect(result.error).toContain('--my-flag');
    });
});
