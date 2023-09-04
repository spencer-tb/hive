// # Test suite for cancun tests
package suite_cancun

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	DATAHASH_START_ADDRESS = big.NewInt(0x100)
	DATAHASH_ADDRESS_COUNT = 1000

	// EIP 4844 specific constants
	GAS_PER_BLOB = uint64(0x20000)

	MIN_DATA_GASPRICE         = uint64(1)
	MAX_BLOB_GAS_PER_BLOCK    = uint64(786432)
	TARGET_BLOB_GAS_PER_BLOCK = uint64(393216)

	TARGET_BLOBS_PER_BLOCK = uint64(TARGET_BLOB_GAS_PER_BLOCK / GAS_PER_BLOB)
	MAX_BLOBS_PER_BLOCK    = uint64(MAX_BLOB_GAS_PER_BLOCK / GAS_PER_BLOB)

	BLOB_GASPRICE_UPDATE_FRACTION = uint64(3338477)

	BLOB_COMMITMENT_VERSION_KZG = byte(0x01)

	// EIP 4788 specific constants
	BEACON_ROOTS_ADDRESS  = common.HexToAddress("0xbEac00dDB15f3B6d645C48263dC93862413A222D")
	HISTORY_BUFFER_LENGTH = uint64(98304)

	// Engine API errors
	INVALID_PARAMS_ERROR   = pInt(-32602)
	UNSUPPORTED_FORK_ERROR = pInt(-38005)
)

// Precalculate the first data gas cost increase
var (
	DATA_GAS_COST_INCREMENT_EXCEED_BLOBS = GetMinExcessBlobsForBlobGasPrice(2)
)

func pUint64(v uint64) *uint64 {
	return &v
}

func pInt(v int) *int {
	return &v
}

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md

// List of all blob tests
var Tests = []test.SpecInterface{
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transactions On Block 1, Shanghai Genesis",
			About: `
			Tests the Cancun fork since Block 1.

			Verifications performed:
			- Correct implementation of Engine API changes for Cancun:
			  - engine_newPayloadV3, engine_forkchoiceUpdatedV3, engine_getPayloadV3
			- Correct implementation of EIP-4844:
			  - Blob transaction ordering and inclusion
			  - Blob transaction blob gas cost checks
			  - Verify Blob bundle on built payload
			- Eth RPC changes for Cancun:
			  - Blob fields in eth_getBlockByNumber
			  - Beacon root in eth_getBlockByNumber
			  - Blob fields in transaction receipts from eth_getTransactionReceipt
			`,
		},

		// We fork on genesis
		CancunForkHeight: 1,

		TestSequence: TestSequence{
			// We are starting at Shanghai genesis so send a couple payloads to reach the fork
			NewPayloads{},

			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				TransactionCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(MAX_BLOBS_PER_BLOCK-TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (MAX_BLOBS_PER_BLOCK - TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transactions On Block 1, Cancun Genesis",
			About: `
			Tests the Cancun fork since genesis.

			Verifications performed:
			* See Blob Transactions On Block 1, Shanghai Genesis
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			NewPayloads{}, // Create a single empty payload to push the client through the fork.
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				TransactionCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(MAX_BLOBS_PER_BLOCK-TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (MAX_BLOBS_PER_BLOCK - TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Single Account",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send N blob transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions
			 - The last payloads must include the last single-blob transactions
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			// Then send the single-blob transactions
			SendBlobTransactions{
				TransactionCount:              MAX_BLOBS_PER_BLOCK + 1,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First four payloads have MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              4,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              2,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Single Account 2",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send a single 2-blob transaction, and send N blob
			transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions
			 - The last payloads must include the rest of the transactions
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the dual-blob transaction
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           2,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the single-blob transactions
			SendBlobTransactions{
				TransactionCount:              MAX_BLOBS_PER_BLOCK - 2,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First five payloads have MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Accounts",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Send N blob transactions with 1 blob each from account B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  0,
			},
			// Then send the single-blob transactions from account B
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Clients",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A, to client A.
			Send N blob transactions with 1 blob each from account B, to client
			B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Start a secondary client to also receive blob transactions
			LaunchClients{
				EngineStarter: hive_rpc.HiveRPCEngineStarter{},
				// Skip adding the second client to the CL Mock to guarantee
				// that all payloads are produced by client A.
				// This is done to not have client B prioritizing single-blob
				// transactions to fill one single payload.
				SkipAddingToCLMock: true,
			},

			// Create a block without any blobs to get past genesis
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: 0,
			},

			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A, to client A.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(120),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			// Then send the single-blob transactions from account B, to client
			// B.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				// Wait a bit more on before requesting the built payload from the client
				GetPayloadDelay: 2,
			},
		},
	},

	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Replace Blob Transactions",
			About: `
			Test sending multiple blob transactions with the same nonce, but
			higher gas tip so the transaction is replaced.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			SendBlobTransactions{ // Blob ID 0
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e9),
				BlobTransactionGasTipCap:      big.NewInt(1e9),
			},
			SendBlobTransactions{ // Blob ID 1
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e2),
				BlobTransactionGasFeeCap:      big.NewInt(1e10),
				BlobTransactionGasTipCap:      big.NewInt(1e10),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 2
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e3),
				BlobTransactionGasFeeCap:      big.NewInt(1e11),
				BlobTransactionGasTipCap:      big.NewInt(1e11),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 3
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e4),
				BlobTransactionGasFeeCap:      big.NewInt(1e12),
				BlobTransactionGasTipCap:      big.NewInt(1e12),
				ReplaceTransactions:           true,
			},

			// We create the first payload, which must contain the blob tx
			// with the higher tip.
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				ExpectedBlobs:             []helper.BlobID{3},
			},
		},
	},

	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Parallel Blob Transactions",
			About: `
			Test sending multiple blob transactions in parallel from different accounts.

			Verify that a payload is created with the maximum number of blobs.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			ParallelSteps{
				Steps: []TestStep{
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  0,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  1,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  2,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  3,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  4,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  5,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  6,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  7,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  8,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  9,
					},
				},
			},

			// We create the first payload, which is guaranteed to have the first MAX_BLOBS_PER_BLOCK blobs.
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, MAX_BLOBS_PER_BLOCK),
			},
		},
	},

	// ForkchoiceUpdatedV3 before cancun
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV3 Set Head to Shanghai Payload, Nil Payload Attributes",
			About: `
			Test sending ForkchoiceUpdatedV3 to set the head of the chain to a Shanghai payload:
			- Send NewPayloadV2 with Shanghai payload on block 1
			- Use ForkchoiceUpdatedV3 to set the head to the payload, with nil payload attributes

			Verify that client returns no error.
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnHeadSet: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{},
				},
				ExpectationDescription: `
				ForkchoiceUpdatedV3 before Cancun returns no error without payload attributes
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV3 To Request Shanghai Payload, Nil Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root is nil

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV3 To Request Shanghai Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							BeaconRoot: &(common.Hash{}),
						},
						ExpectedError: UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV3 before Cancun with beacon root must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV2 before cancun with beacon root
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV2 To Request Shanghai Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							BeaconRoot: &(common.Hash{}),
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 before Cancun with beacon root field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV2 after cancun
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV2 To Request Cancun Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Cancun timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 after Cancun with beacon root field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV2 To Request Cancun Payload, Nil Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Cancun timestamp
			- Payload Attributes' Beacon Root nil (not provided)

			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							RemoveBeaconRoot: true,
						},
						ExpectedError: UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 after Cancun must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV3 with modified BeaconRoot Attribute
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV3 Modifies Payload ID on Different Beacon Root",
			About: `
			Test requesting a Cancun Payload using ForkchoiceUpdatedV3 twice with the beacon root
			payload attribute as the only change between requests and verify that the payload ID is
			different.
			`,
		},

		CancunForkHeight: 0,

		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
						BeaconRoot: &(common.Hash{}),
					},
				},
			},
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
						BeaconRoot: &(common.Hash{1}),
					},
				},
			},
		},
	},

	// GetPayloadV3 Before Cancun, Negative Tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "GetPayloadV3 To Request Shanghai Payload",
			About: `
			Test requesting a Shanghai PayloadID using GetPayloadV3.
			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				GetPayloadCustomizer: &helper.UpgradeGetPayloadVersion{
					GetPayloadCustomizer: &helper.BaseGetPayloadCustomizer{
						ExpectedError: UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				GetPayloadV3 To Request Shanghai Payload must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// GetPayloadV2 After Cancun, Negative Tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "GetPayloadV2 To Request Cancun Payload",
			About: `
			Test requesting a Cancun PayloadID using GetPayloadV2.
			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				GetPayloadCustomizer: &helper.DowngradeGetPayloadVersion{
					GetPayloadCustomizer: &helper.BaseGetPayloadCustomizer{
						ExpectedError: UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				GetPayloadV2 To Request Cancun Payload must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// NewPayloadV3 Before Cancun, Negative Tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root

			Verify that client returns INVALID_PARAMS_ERROR
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: nil,
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							BlobGasUsed: pUint64(0),
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ExcessBlobGas: pUint64(0),
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Empty Array Versioned Hashes, Nil Beacon Root",
			About: `
				Test sending NewPayloadV3 Before Cancun with:
				- nil ExcessBlobGas
				- nil BlobGasUsed
				- Empty Versioned Hashes Array
				- nil Beacon Root
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{},
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- Zero Beacon Root
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ParentBeaconRoot: &(common.Hash{}),
						},
						ExpectedError: INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, 0x00 Data Fields, Empty Array Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			- Zero Beacon Root
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ExcessBlobGas:    pUint64(0),
							BlobGasUsed:      pUint64(0),
							ParentBeaconRoot: &(common.Hash{}),
						},
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{},
						},
						ExpectedError: UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with no nil fields must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// NewPayloadV3 After Cancun, Negative Tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 After Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Empty Array Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			- Zero Beacon Root
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveExcessBlobGas: true,
					},
					ExpectedError: INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil ExcessBlobGas must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 After Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Empty Array Versioned Hashes",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- Empty Versioned Hashes Array
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveBlobGasUsed: true,
					},
					ExpectedError: INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil BlobGasUsed must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 After Cancun, 0x00 Blob Fields, Empty Array Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- Empty Versioned Hashes Array
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveParentBeaconRoot: true,
					},
					ExpectedError: INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil parentBeaconBlockRoot must return INVALID_PARAMS_ERROR (code %d)
				`, *INVALID_PARAMS_ERROR),
			},
		},
	},

	// Fork time tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "ForkchoiceUpdatedV2 then ForkchoiceUpdatedV3 Valid Payload Building Requests",
			About: `
			Test requesting a Shanghai ForkchoiceUpdatedV2 payload followed by a Cancun ForkchoiceUpdatedV3 request.
			Verify that client correctly returns the Cancun payload.
			`,
		},

		// We request two blocks from the client, first on shanghai and then on cancun, both with
		// the same parent.
		// Client must respond correctly to later request.
		CancunForkHeight: 1,
		TimeIncrements:   2,

		TestSequence: TestSequence{
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				// This customizer only simulates requesting a Shanghai payload 1 second before cancun.
				// CL Mock will still request the Cancun payload afterwards
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.TimestampDeltaPayloadAttributesCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							RemoveBeaconRoot: true,
						},
						TimestampDelta: -1,
					},
				},
				ExpectationDescription: `
				ForkchoiceUpdatedV3 must construct transaction with blob payloads even if a ForkchoiceUpdatedV2 was previously requested
				`,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is missing one of the hashes.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1),
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK+1),
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobListByIndex(helper.BlobID(TARGET_BLOBS_PER_BLOCK-1), 0),
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK), helper.BlobID(TARGET_BLOBS_PER_BLOCK-1)),
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob hash that does not belong to any blob contained in the payload.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(TARGET_BLOBS_PER_BLOCK)),
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect hash in list of versioned hashes must return INVALID status
				`,
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs:        helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
						HashVersions: []byte{BLOB_COMMITMENT_VERSION_KZG, BLOB_COMMITMENT_VERSION_KZG + 1},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect version in list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: nil,
					},
					ExpectedError: INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: `
				NewPayloadV3 after Cancun with nil VersionedHashes must return INVALID_PARAMS_ERROR (code -32602)
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: []helper.BlobID{},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				ExpectedBlobs: []helper.BlobID{},
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: []helper.BlobID{0},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3 on syncing clients
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash (Syncing)",
			About: `
				Tests VersionedHashes in Engine API NewPayloadV3 where the array
				is missing one of the hashes.
				`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK+1),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},
			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: helper.GetBlobListByIndex(helper.BlobID(TARGET_BLOBS_PER_BLOCK-1), 0),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK), helper.BlobID(TARGET_BLOBS_PER_BLOCK-1)),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(TARGET_BLOBS_PER_BLOCK)),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs:        helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
						HashVersions: []byte{BLOB_COMMITMENT_VERSION_KZG, BLOB_COMMITMENT_VERSION_KZG + 1},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: nil,
					},
					ExpectedError: INVALID_PARAMS_ERROR,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: []helper.BlobID{},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			NewPayloads{
				ExpectedBlobs: []helper.BlobID{},
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					VersionedHashesCustomizer: &VersionedHashes{
						Blobs: []helper.BlobID{0},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	// BlobGasUsed, ExcessBlobGas Negative Tests
	// Most cases are contained in https://github.com/ethereum/execution-spec-tests/tree/main/tests/cancun/eip4844_blobs
	// and can be executed using `pyspec` simulator.
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Incorrect BlobGasUsed: Non-Zero on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						BlobGasUsed: pUint64(1),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{

		Spec: test.Spec{
			Name: "Incorrect BlobGasUsed: GAS_PER_BLOB on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						BlobGasUsed: pUint64(GAS_PER_BLOB),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	// ForkID tests
	&CancunForkSpec{
		GenesisTimestamp:  0,
		ShanghaiTimestamp: 0,
		CancunTimestamp:   0,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 0, shanghai at 0, cancun at 0",
				About: `
			Attemp to peer client with the following configuration at height 0:
			- genesis timestamp 0
			- shanghai fork at timestamp 0
			- cancun fork at timestamp 0
			`,
			},
		},
	},
	&CancunForkSpec{
		GenesisTimestamp:  0,
		ShanghaiTimestamp: 0,
		CancunTimestamp:   1,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 0, shanghai at 0, cancun at 1",
				About: `
			Attemp to peer client with the following configuration at height 0:
			- genesis timestamp 0
			- shanghai fork at timestamp 0
			- cancun fork at timestamp 1
			`,
			},
		},
	},

	&CancunForkSpec{
		GenesisTimestamp:  1,
		ShanghaiTimestamp: 0,
		CancunTimestamp:   1,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 1, shanghai at 0, cancun at 1",
				About: `
			Attemp to peer client with the following configuration at height 0:
			- genesis timestamp 1
			- shanghai fork at timestamp 0
			- cancun fork at timestamp 1
			`,
			},
		},
	},

	&CancunForkSpec{
		GenesisTimestamp:           0,
		ShanghaiTimestamp:          0,
		CancunTimestamp:            1,
		ProduceBlocksBeforePeering: 1,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 0, shanghai at 0, cancun at 1, transition",
				About: `
			Attemp to peer client with the following configuration at height 1:
			- genesis timestamp 0
			- shanghai fork at timestamp 0
			- cancun fork at timestamp 1
			`,
			},
		},
	},

	&CancunForkSpec{
		GenesisTimestamp:  1,
		ShanghaiTimestamp: 1,
		CancunTimestamp:   1,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 1, shanghai at 1, cancun at 1",
				About: `
			Attemp to peer client with the following configuration at height 0:
			- genesis timestamp 1
			- shanghai fork at timestamp 1
			- cancun fork at timestamp 1
			`,
			},
		},
	},
	&CancunForkSpec{
		GenesisTimestamp:  1,
		ShanghaiTimestamp: 1,
		CancunTimestamp:   2,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 1, shanghai at 1, cancun at 2",
				About: `
			Attemp to peer client with the following configuration at height 0:
			- genesis timestamp 1
			- shanghai fork at timestamp 1
			- cancun fork at timestamp 2
			`,
			},
		},
	},
	&CancunForkSpec{
		GenesisTimestamp:           1,
		ShanghaiTimestamp:          1,
		CancunTimestamp:            2,
		ProduceBlocksBeforePeering: 1,

		CancunBaseSpec: CancunBaseSpec{
			Spec: test.Spec{
				Name: "ForkID, genesis at 1, shanghai at 1, cancun at 2, transition",
				About: `
			Attemp to peer client with the following configuration at height 1:
			- genesis timestamp 1
			- shanghai fork at timestamp 1
			- cancun fork at timestamp 2
			`,
			},
		},
	},

	// DevP2P tests
	&CancunBaseSpec{
		Spec: test.Spec{
			Name: "Request Blob Pooled Transactions",
			About: `
			Requests blob pooled transactions and verify correct encoding.
			`,
		},
		TestSequence: TestSequence{
			// Get past the genesis
			NewPayloads{
				PayloadCount: 1,
			},
			// Send multiple transactions with multiple blobs each
			SendBlobTransactions{
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			DevP2PRequestPooledTransactionHash{
				ClientIndex:                 0,
				TransactionIndexes:          []uint64{0},
				WaitForNewPooledTransaction: true,
			},
		},
	},
}
