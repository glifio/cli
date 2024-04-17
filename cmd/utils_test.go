package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/jimpick/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestParseAddress(t *testing.T) {
	// Create a mock FullNodeAPI
	mockAPI := &MockFullNodeAPI{}

	testCases := []struct {
		name        string
		input       string
		expected    common.Address
		expectError bool
	}{
		{
			name:        "Valid Ethereum Address",
			input:       "0x3972E844729522d367BFA1D64368346D7ccEEa59",
			expected:    common.HexToAddress("0x3972E844729522d367BFA1D64368346D7ccEEa59"),
			expectError: false,
		},
		{
			name:        "Valid Filecoin ID Address",
			input:       idStr,
			expected:    common.HexToAddress(maskedIDStr),
			expectError: false,
		},
		{
			name:        "Valid Filecoin Account f1 Address",
			input:       "f1ys5qqiciehcml3sp764ymbbytfn3qoar5fo3iwy",
			expected:    common.HexToAddress(maskedIDStr),
			expectError: false,
		},
		{
			name:        "Valid Filecoin Account f3 Address",
			input:       "f3vpyybzycb3wvhwkxcrodn3rqv66sd5hfho4lfq6p6igmrlgyb22v3ekdghp6km47ioki3gfo4zb4ezirhfaq",
			expected:    common.HexToAddress(maskedIDStr),
			expectError: false,
		},
		{
			name:        "Valid Filecoin Account f4 Address",
			input:       "f410fmdqxonrwz5peuit5tlbe6ih6zibu5ys223xctfi",
			expected:    common.HexToAddress("0x60E1773636CF5E4A227d9AC24F20fEca034ee25A"),
			expectError: false,
		},
		{
			name:        "Invalid Address",
			input:       "invalid_address",
			expectError: true,
		},
		{
			name:        "EVM Actor",
			input:       evmActorID,
			expectError: true,
		},
		{
			name:        "Eth Account",
			input:       ethAccountID,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAddress(context.Background(), tc.input, mockAPI)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				fmt.Println(result.String())
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
