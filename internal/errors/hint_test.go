// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Recovery/compatibility failures should carry actionable remediation guidance.
func TestRecoveryWrappersAttachHints(t *testing.T) {
	cases := map[string]error{
		"transaction not found": WrapTransactionNotFound(errors.New("404")),
		"rpc connection failed": WrapRPCConnectionFailed(errors.New("dial tcp")),
		"rpc timeout":           WrapRPCTimeout(errors.New("context deadline exceeded")),
		"protocol unsupported":  WrapProtocolUnsupported(99),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			hint := Hint(err)
			assert.NotEmpty(t, hint, "expected an actionable hint for %s", name)
			// A hint should be guidance, not just a restatement of the error code.
			assert.Greater(t, len(hint), 20, "hint should be a real, actionable message")
		})
	}
}

// Hint returns "" for errors that carry no guidance and for plain errors.
func TestHintEmptyWhenAbsent(t *testing.T) {
	assert.Empty(t, Hint(WrapMarshalFailed(errors.New("boom"))), "wrapper without a hint should return no hint")
	assert.Empty(t, Hint(errors.New("plain error")), "non-ErstError should return no hint")
	assert.Empty(t, Hint(nil), "nil error should return no hint")
}

// WithHint sets the hint, is chainable, and is reachable through error wrapping.
func TestWithHintChainableAndUnwrappable(t *testing.T) {
	base := WrapValidationError("bad input").(*ErstError).WithHint("pass --foo")
	assert.Equal(t, "pass --foo", Hint(base))

	wrapped := errors.New("outer: " + base.Error())
	_ = wrapped // Error() text is unchanged; the hint travels on the typed error, below.

	// The hint must be discoverable via errors.As through a wrapping chain.
	chain := &ErstError{Code: ErstValidationFailed, Message: "x", OrigErr: base, Hint: "do the thing"}
	assert.Equal(t, "do the thing", Hint(chain))
}

// Attaching a hint must not change the user-facing Error() string, so existing
// callers and tests that assert on Error() are unaffected (backward compatible).
func TestHintDoesNotLeakIntoErrorString(t *testing.T) {
	err := WrapRPCConnectionFailed(errors.New("dial tcp"))
	assert.False(t, strings.Contains(err.Error(), Hint(err)),
		"Error() should not include the hint text")
	// Sentinel matching still works on a hinted error.
	assert.True(t, errors.Is(err, ErrRPCConnectionFailed))
}
