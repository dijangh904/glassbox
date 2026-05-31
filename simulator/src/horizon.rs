// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

use chrono::{DateTime, Utc};
use serde::Deserialize;

/// Raw transaction record returned by the Horizon API.
#[derive(Debug, Deserialize)]
pub struct HorizonTransaction {
    pub hash: String,
    pub successful: bool,
    pub created_at: String,
}

/// A transaction enriched with a parsed timestamp, ready for alert evaluation.
#[derive(Debug)]
pub struct EnrichedTransaction {
    pub hash: String,
    pub successful: bool,
    pub created_at: DateTime<Utc>,
}

impl EnrichedTransaction {
    /// Build an `EnrichedTransaction` from a raw Horizon record.
    ///
    /// If `created_at` cannot be parsed as RFC 3339, a warning is logged and
    /// `Utc::now()` is used as the fallback so the transaction is still
    /// evaluated rather than silently dropped.
    pub fn from_horizon(tx: HorizonTransaction) -> Self {
        let created_at = DateTime::parse_from_rfc3339(&tx.created_at)
            .map(|dt| dt.with_timezone(&Utc))
            .unwrap_or_else(|err| {
                tracing::warn!(
                    hash = %tx.hash,
                    raw = %tx.created_at,
                    %err,
                    "failed to parse created_at; falling back to Utc::now()"
                );
                Utc::now()
            });

        Self {
            hash: tx.hash,
            successful: tx.successful,
            created_at,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_timestamp_is_parsed() {
        let tx = HorizonTransaction {
            hash: "abc".into(),
            successful: true,
            created_at: "2024-01-15T10:30:00Z".into(),
        };
        let enriched = EnrichedTransaction::from_horizon(tx);
        assert_eq!(enriched.hash, "abc");
        assert_eq!(enriched.created_at.to_rfc3339(), "2024-01-15T10:30:00+00:00");
    }

    #[test]
    fn bad_timestamp_falls_back_to_now_and_transaction_is_still_returned() {
        let before = Utc::now();
        let tx = HorizonTransaction {
            hash: "bad-ts".into(),
            successful: false,
            created_at: "not-a-date".into(),
        };
        let enriched = EnrichedTransaction::from_horizon(tx);
        let after = Utc::now();

        // Transaction is still returned (not dropped).
        assert_eq!(enriched.hash, "bad-ts");
        assert!(!enriched.successful);
        // Fallback timestamp is within the test window.
        assert!(enriched.created_at >= before);
        assert!(enriched.created_at <= after);
    }
}
