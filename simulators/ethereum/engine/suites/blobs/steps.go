package suite_blobs

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type BlobTestContext struct {
	*test.Env
	*TestBlobTxPool
}

// Interface to represent a single step in a test vector
type TestStep interface {
	// Executes the step
	Execute(testCtx *BlobTestContext) error
	Description() string
}

type TestSequence []TestStep

// A step that runs two or more steps in parallel
type ParallelSteps struct {
	Steps []TestStep
}

func (step ParallelSteps) Execute(t *BlobTestContext) error {
	// Run the steps in parallel
	wg := sync.WaitGroup{}
	errs := make(chan error, len(step.Steps))
	for _, s := range step.Steps {
		wg.Add(1)
		go func(s TestStep) {
			defer wg.Done()
			if err := s.Execute(t); err != nil {
				errs <- err
			}
		}(s)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func (step ParallelSteps) Description() string {
	desc := fmt.Sprintf("ParallelSteps: running steps in parallel:\n")
	for i, step := range step.Steps {
		desc += fmt.Sprintf("%d: %s\n", i, step.Description())
	}

	return desc
}

// A step that launches a new client
type LaunchClients struct {
	client.EngineStarter
	ClientCount              uint64
	SkipConnectingToBootnode bool
	SkipAddingToCLMock       bool
}

func (step LaunchClients) GetClientCount() uint64 {
	clientCount := step.ClientCount
	if clientCount == 0 {
		clientCount = 1
	}
	return clientCount
}

func (step LaunchClients) Execute(t *BlobTestContext) error {
	// Launch a new client
	var (
		client client.EngineClient
		err    error
	)
	clientCount := step.GetClientCount()
	for i := uint64(0); i < clientCount; i++ {
		if !step.SkipConnectingToBootnode {
			client, err = step.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engines[0])
		} else {
			client, err = step.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
		}
		if err != nil {
			return err
		}
		t.Engines = append(t.Engines, client)
		t.TestEngines = append(t.TestEngines, test.NewTestEngineClient(t.Env, client))
		if !step.SkipAddingToCLMock {
			t.CLMock.AddEngineClient(client)
		}
	}
	return nil
}

func (step LaunchClients) Description() string {
	return fmt.Sprintf("Launch %d new engine client(s)", step.GetClientCount())
}

// A step that sends a new payload to the client
type NewPayloads struct {
	// Payload Count
	PayloadCount uint64
	// Number of blob transactions that are expected to be included in the payload
	ExpectedIncludedBlobCount uint64
	// Blob IDs expected to be found in the payload
	ExpectedBlobs []helper.BlobID
	// Delay between FcU and GetPayload calls
	GetPayloadDelay uint64
	// Extra modifications on NewPayload to the versioned hashes
	VersionedHashes *VersionedHashes
	// Extra modifications on NewPayload to potentially generate an invalid payload
	PayloadCustomizer helper.PayloadCustomizer
	// Version to use to call NewPayload
	Version uint64
	// Expected responses on the NewPayload call
	ExpectedError  *int
	ExpectedStatus test.PayloadStatus
}

type VersionedHashes struct {
	Blobs        []helper.BlobID
	HashVersions []byte
}

func (v *VersionedHashes) VersionedHashes() ([]common.Hash, error) {
	if v.Blobs == nil {
		return nil, nil
	}

	versionedHashes := make([]common.Hash, len(v.Blobs))

	for i, blobID := range v.Blobs {
		var version byte
		if v.HashVersions != nil && len(v.HashVersions) > i {
			version = v.HashVersions[i]
		}
		var err error
		versionedHashes[i], err = blobID.GetVersionedHash(version)
		if err != nil {
			return nil, err
		}

	}

	return versionedHashes, nil
}

func (v *VersionedHashes) Description() string {
	desc := "VersionedHashes: "
	if v.Blobs != nil {
		desc += fmt.Sprintf("%v", v.Blobs)
	}
	if v.HashVersions != nil {
		desc += fmt.Sprintf(" with versions %v", v.HashVersions)
	}
	return desc

}

func (step NewPayloads) GetPayloadCount() uint64 {
	payloadCount := step.PayloadCount
	if payloadCount == 0 {
		payloadCount = 1
	}
	return payloadCount
}

type BlobWrapData struct {
	VersionedHash common.Hash
	KZG           typ.KZGCommitment
	Blob          typ.Blob
	Proof         typ.KZGProof
}

func GetBlobDataInPayload(pool *TestBlobTxPool, payload *engine.ExecutableData) ([]*typ.TransactionWithBlobData, []*BlobWrapData, error) {
	// Find all blob transactions included in the payload
	var (
		blobDataInPayload = make([]*BlobWrapData, 0)
		blobTxsInPayload  = make([]*typ.TransactionWithBlobData, 0)
	)
	signer := types.NewCancunSigner(globals.ChainID)

	for i, binaryTx := range payload.Transactions {
		// Unmarshal the tx from the payload, which should be the minimal version
		// of the blob transaction
		txData := new(types.Transaction)
		if err := txData.UnmarshalBinary(binaryTx); err != nil {
			return nil, nil, err
		}

		if txData.Type() != types.BlobTxType {
			continue
		}

		// Print transaction info
		sender, err := signer.Sender(txData)
		if err != nil {
			return nil, nil, err
		}
		fmt.Printf("Tx %d in the payload: From: %s, Nonce: %d\n", i, sender, txData.Nonce())

		// Find the transaction in the current pool of known transactions
		if tx, ok := pool.Transactions[txData.Hash()]; ok {
			if blobTx, ok := tx.(*typ.TransactionWithBlobData); ok {
				if blobTx.BlobData == nil {
					return nil, nil, fmt.Errorf("blob data is nil")
				}
				var (
					kzgs            = blobTx.BlobData.Commitments
					blobs           = blobTx.BlobData.Blobs
					proofs          = blobTx.BlobData.Proofs
					versionedHashes = blobTx.BlobHashes()
				)

				if len(versionedHashes) != len(kzgs) || len(kzgs) != len(blobs) || len(blobs) != len(proofs) {
					return nil, nil, fmt.Errorf("invalid blob wrap data")
				}
				for i := 0; i < len(versionedHashes); i++ {
					blobDataInPayload = append(blobDataInPayload, &BlobWrapData{
						VersionedHash: versionedHashes[i],
						KZG:           kzgs[i],
						Blob:          blobs[i],
						Proof:         proofs[i],
					})
				}
				blobTxsInPayload = append(blobTxsInPayload, blobTx)
			} else {
				return nil, nil, fmt.Errorf("could not find blob data in transaction %s, type=%T", txData.Hash().String(), tx)
			}

		} else {
			return nil, nil, fmt.Errorf("could not find transaction %s in the pool", txData.Hash().String())
		}
	}
	return blobTxsInPayload, blobDataInPayload, nil
}

func (step NewPayloads) VerifyPayload(ctx context.Context, forkConfig *globals.ForkConfig, testEngine *test.TestEngineClient, blobTxsInPayload []*typ.TransactionWithBlobData, payload *engine.ExecutableData, previousPayload *engine.ExecutableData) error {
	var (
		parentExcessDataGas = uint64(0)
		parentDataGasUsed   = uint64(0)
	)
	if previousPayload != nil {
		if previousPayload.ExcessDataGas != nil {
			parentExcessDataGas = *previousPayload.ExcessDataGas
		}
		if previousPayload.DataGasUsed != nil {
			parentDataGasUsed = *previousPayload.DataGasUsed
		}
	}
	expectedExcessDataGas := CalcExcessDataGas(parentExcessDataGas, parentDataGasUsed)

	// TODO: check whether the new payload should be in cancun or not
	if forkConfig.IsCancun(payload.Timestamp) {
		if payload.ExcessDataGas == nil {
			return fmt.Errorf("payload contains nil excessDataGas")
		}
		if payload.DataGasUsed == nil {
			return fmt.Errorf("payload contains nil dataGasUsed")
		}
		if *payload.ExcessDataGas != expectedExcessDataGas {
			return fmt.Errorf("payload contains incorrect excessDataGas: want 0x%x, have 0x%x", expectedExcessDataGas, *payload.ExcessDataGas)
		}

		totalBlobCount := uint64(0)
		expectedDataGasPrice := GetDataGasPrice(expectedExcessDataGas)

		for _, tx := range blobTxsInPayload {
			blobCount := uint64(len(tx.BlobHashes()))
			totalBlobCount += blobCount

			// Retrieve receipt from client
			r := testEngine.TestTransactionReceipt(tx.Hash())
			expectedDataGasUsed := blobCount * DATA_GAS_PER_BLOB
			r.ExpectDataGasUsed(&expectedDataGasUsed)
			r.ExpectDataGasPrice(&expectedDataGasPrice)
		}

		if totalBlobCount != step.ExpectedIncludedBlobCount {
			return fmt.Errorf("expected %d blobs in transactions, got %d", step.ExpectedIncludedBlobCount, totalBlobCount)
		}

	} else {
		if payload.ExcessDataGas != nil {
			return fmt.Errorf("payload contains non-nil excessDataGas pre-fork")
		}
		if payload.DataGasUsed != nil {
			return fmt.Errorf("payload contains non-nil dataGasUsed pre-fork")
		}
	}

	return nil
}

func (step NewPayloads) VerifyBlobBundle(blobDataInPayload []*BlobWrapData, payload *engine.ExecutableData, blobBundle *typ.BlobsBundle) error {

	if len(blobBundle.Blobs) != len(blobBundle.Commitments) || len(blobBundle.Blobs) != len(blobBundle.Proofs) {
		return fmt.Errorf("unexpected length in blob bundle: %d blobs, %d proofs, %d commitments", len(blobBundle.Blobs), len(blobBundle.Proofs), len(blobBundle.Commitments))
	}
	if len(blobBundle.Blobs) != int(step.ExpectedIncludedBlobCount) {
		return fmt.Errorf("expected %d blob, got %d", step.ExpectedIncludedBlobCount, len(blobBundle.Blobs))
	}

	// Verify that the calculated amount of blobs in the payload matches the
	// amount of blobs in the bundle
	if len(blobDataInPayload) != len(blobBundle.Blobs) {
		return fmt.Errorf("expected %d blobs in the bundle, got %d", len(blobDataInPayload), len(blobBundle.Blobs))
	}

	for i, blobData := range blobDataInPayload {
		bundleCommitment := blobBundle.Commitments[i]
		bundleBlob := blobBundle.Blobs[i]
		bundleProof := blobBundle.Proofs[i]
		if !bytes.Equal(bundleCommitment[:], blobData.KZG[:]) {
			return fmt.Errorf("KZG mismatch at index %d of the bundle", i)
		}
		if !bytes.Equal(bundleBlob[:], blobData.Blob[:]) {
			return fmt.Errorf("blob mismatch at index %d of the bundle", i)
		}
		if !bytes.Equal(bundleProof[:], blobData.Proof[:]) {
			return fmt.Errorf("proof mismatch at index %d of the bundle", i)
		}
	}

	if len(step.ExpectedBlobs) != 0 {
		// Verify that the blobs in the payload match the expected blobs
		for _, expectedBlob := range step.ExpectedBlobs {
			found := false
			for _, blobData := range blobDataInPayload {
				if ok, err := expectedBlob.VerifyBlob(&blobData.Blob); err != nil {
					return err
				} else if ok {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("could not find expected blob %d", expectedBlob)
			}
		}
	}

	return nil
}

func (step NewPayloads) Execute(t *BlobTestContext) error {
	// Create a new payload
	// Produce the payload
	payloadCount := step.GetPayloadCount()

	var originalGetPayloadDelay time.Duration
	if step.GetPayloadDelay != 0 {
		originalGetPayloadDelay = t.CLMock.PayloadProductionClientDelay
		t.CLMock.PayloadProductionClientDelay = time.Duration(step.GetPayloadDelay) * time.Second
	}
	var (
		previousPayload = t.CLMock.LatestPayloadBuilt
	)
	for p := uint64(0); p < payloadCount; p++ {
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnGetPayload: func() {
				// Get the latest blob bundle
				var (
					blobBundle = t.CLMock.LatestBlobBundle
					payload    = &t.CLMock.LatestPayloadBuilt
				)

				if !t.Env.ForkConfig.IsCancun(payload.Timestamp) {
					// Nothing to do
					return
				}
				if blobBundle == nil {
					t.Fatalf("FAIL: Error getting blobs bundle (payload %d/%d): %v", p+1, payloadCount, blobBundle)
				}

				_, blobDataInPayload, err := GetBlobDataInPayload(t.TestBlobTxPool, payload)
				if err != nil {
					t.Fatalf("FAIL: Error retrieving blob bundle (payload %d/%d): %v", p+1, payloadCount, err)
				}

				if err := step.VerifyBlobBundle(blobDataInPayload, payload, blobBundle); err != nil {
					t.Fatalf("FAIL: Error verifying blob bundle (payload %d/%d): %v", p+1, payloadCount, err)
				}
			},
			OnNewPayloadBroadcast: func() {
				// Send a test NewPayload directive with either a modified payload or modifed versioned hashes
				var (
					payload                       = &t.CLMock.LatestPayloadBuilt
					versionedHashes []common.Hash = nil
					r               *test.NewPayloadResponseExpectObject
					err             error
				)
				if step.VersionedHashes != nil {
					// Send a new payload with the modified versioned hashes
					versionedHashes, err = step.VersionedHashes.VersionedHashes()
					if err != nil {
						t.Fatalf("FAIL: Error getting modified versioned hashes (payload %d/%d): %v", p+1, payloadCount, err)
					}
				} else {
					if t.CLMock.LatestBlobBundle != nil {
						versionedHashes, err = t.CLMock.LatestBlobBundle.VersionedHashes(BLOB_COMMITMENT_VERSION_KZG)
						if err != nil {
							t.Fatalf("FAIL: Error getting versioned hashes (payload %d/%d): %v", p+1, payloadCount, err)
						}
					}
				}

				if step.PayloadCustomizer != nil {
					// Send a custom new payload
					payload, err = step.PayloadCustomizer.CustomizePayload(payload)
					if err != nil {
						t.Fatalf("FAIL: Error customizing payload (payload %d/%d): %v", p+1, payloadCount, err)
					}
				}

				version := step.Version
				if version == 0 {
					if t.Env.ForkConfig.IsCancun(payload.Timestamp) {
						version = 3
					} else {
						version = 2
					}
				}

				if version == 3 {
					r = t.TestEngine.TestEngineNewPayloadV3(payload, versionedHashes)
				} else if version == 2 {
					r = t.TestEngine.TestEngineNewPayloadV2(payload)
				} else {
					t.Fatalf("FAIL: Unknown version %d", step.Version)
				}
				if step.ExpectedError != nil {
					r.ExpectErrorCode(*step.ExpectedError)
				} else {
					r.ExpectNoError()
					if step.ExpectedStatus != "" {
						r.ExpectStatus(step.ExpectedStatus)
					}
				}
			},
			OnForkchoiceBroadcast: func() {
				// Verify the transaction receipts on incorporated transactions
				payload := &t.CLMock.LatestPayloadBuilt

				blobTxsInPayload, _, err := GetBlobDataInPayload(t.TestBlobTxPool, payload)
				if err != nil {
					t.Fatalf("FAIL: Error retrieving blob bundle (payload %d/%d): %v", p+1, payloadCount, err)
				}
				if err := step.VerifyPayload(t.TimeoutContext, t.Env.ForkConfig, t.TestEngine, blobTxsInPayload, payload, &previousPayload); err != nil {
					t.Fatalf("FAIL: Error verifying payload (payload %d/%d): %v", p+1, payloadCount, err)
				}
				previousPayload = t.CLMock.LatestPayloadBuilt
			},
		})
		t.Logf("INFO: Correctly produced payload %d/%d", p+1, payloadCount)
	}
	if step.GetPayloadDelay != 0 {
		// Restore the original delay
		t.CLMock.PayloadProductionClientDelay = originalGetPayloadDelay
	}
	return nil
}

func (step NewPayloads) Description() string {
	if step.VersionedHashes != nil {
		return fmt.Sprintf("NewPayloads: %d payloads, %d blobs expected, %s", step.GetPayloadCount(), step.ExpectedIncludedBlobCount, step.VersionedHashes.Description())
	}
	return fmt.Sprintf("NewPayloads: %d payloads, %d blobs expected", step.GetPayloadCount(), step.ExpectedIncludedBlobCount)
}

// A step that sends multiple new blobs to the client
type SendBlobTransactions struct {
	// Number of blob transactions to send before this block's GetPayload request
	BlobTransactionSendCount uint64
	// Blobs per transaction
	BlobsPerTransaction uint64
	// Max Data Gas Cost for every blob transaction
	BlobTransactionMaxDataGasCost *big.Int
	// Gas Fee Cap for every blob transaction
	BlobTransactionGasFeeCap *big.Int
	// Gas Tip Cap for every blob transaction
	BlobTransactionGasTipCap *big.Int
	// Replace transactions
	ReplaceTransactions bool
	// Skip verification of retrieving the tx from node
	SkipVerificationFromNode bool
	// Account index to send the blob transactions from
	AccountIndex uint64
	// Client index to send the blob transactions to
	ClientIndex uint64
}

func (step SendBlobTransactions) GetBlobsPerTransaction() uint64 {
	blobCountPerTx := step.BlobsPerTransaction
	if blobCountPerTx == 0 {
		blobCountPerTx = 1
	}
	return blobCountPerTx
}

func (step SendBlobTransactions) Execute(t *BlobTestContext) error {
	// Send a blob transaction
	addr := common.BigToAddress(DATAHASH_START_ADDRESS)
	blobCountPerTx := step.GetBlobsPerTransaction()
	var engine client.EngineClient
	if step.ClientIndex >= uint64(len(t.Engines)) {
		return fmt.Errorf("invalid client index %d", step.ClientIndex)
	}
	engine = t.Engines[step.ClientIndex]
	//  Send the blob transactions
	for bTx := uint64(0); bTx < step.BlobTransactionSendCount; bTx++ {
		blobTxCreator := &helper.BlobTransactionCreator{
			To:         &addr,
			GasLimit:   100000,
			GasTip:     step.BlobTransactionGasTipCap,
			GasFee:     step.BlobTransactionGasFeeCap,
			DataGasFee: step.BlobTransactionMaxDataGasCost,
			BlobCount:  blobCountPerTx,
			BlobID:     t.CurrentBlobID,
		}
		if step.AccountIndex != 0 {
			if step.AccountIndex >= uint64(len(globals.TestAccounts)) {
				return fmt.Errorf("invalid account index %d", step.AccountIndex)
			}
			key := globals.TestAccounts[step.AccountIndex].GetKey()
			blobTxCreator.PrivateKey = key
		}
		var (
			blobTx typ.Transaction
			err    error
		)
		if step.ReplaceTransactions {
			blobTx, err = helper.ReplaceLastTransaction(t.TestContext, engine,
				blobTxCreator,
			)
		} else {
			blobTx, err = helper.SendNextTransaction(t.TestContext, engine,
				blobTxCreator,
			)
		}
		if err != nil {
			t.Fatalf("FAIL: Error sending blob transaction: %v", err)
		}
		if !step.SkipVerificationFromNode {
			VerifyTransactionFromNode(t.TestContext, engine, blobTx)
		}
		t.TestBlobTxPool.Mutex.Lock()
		t.AddBlobTransaction(blobTx)
		t.HashesByIndex[t.CurrentTransactionIndex] = blobTx.Hash()
		t.CurrentTransactionIndex += 1
		t.Logf("INFO: Sent blob transaction: %s", blobTx.Hash().String())
		t.CurrentBlobID += helper.BlobID(blobCountPerTx)
		t.TestBlobTxPool.Mutex.Unlock()
	}
	return nil
}

func (step SendBlobTransactions) Description() string {
	return fmt.Sprintf("SendBlobTransactions: %d Transactions, %d blobs each, %d max data gas fee", step.BlobTransactionSendCount, step.GetBlobsPerTransaction(), step.BlobTransactionMaxDataGasCost.Uint64())
}

// Send a modified version of the latest payload produced using NewPayloadV3
type SendModifiedLatestPayload struct {
	ClientID uint64
	// Versioned hashes modification
	VersionedHashes *VersionedHashes
	// Expected status of the new payload request
	ExpectedStatus test.PayloadStatus
}

func (step SendModifiedLatestPayload) Execute(t *BlobTestContext) error {
	// Get the latest payload
	payload := &t.CLMock.LatestPayloadBuilt
	if payload == nil {
		return fmt.Errorf("no payload available")
	}
	// Modify the versioned hashes
	versionedHashes, err := step.VersionedHashes.VersionedHashes()
	if err != nil {
		return fmt.Errorf("error getting modified versioned hashes: %v", err)
	}
	// Send the payload
	if step.ClientID >= uint64(len(t.TestEngines)) {
		return fmt.Errorf("invalid client index %d", step.ClientID)
	}
	testEngine := t.TestEngines[step.ClientID]
	r := testEngine.TestEngineNewPayloadV3(payload, versionedHashes)
	r.ExpectStatus(step.ExpectedStatus)
	return nil
}

func (step SendModifiedLatestPayload) Description() string {
	desc := fmt.Sprintf("SendModifiedLatestPayload: client %d, expected status %s, ", step.ClientID, step.ExpectedStatus)
	if step.VersionedHashes != nil {
		desc += step.VersionedHashes.Description()
	}

	return desc
}
