package helper

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	"github.com/pkg/errors"

	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

var gCryptoCtx gokzg4844.Context
var initCryptoCtx sync.Once

// InitializeCryptoCtx initializes the global context object returned via CryptoCtx
func InitializeCryptoCtx() {
	initCryptoCtx.Do(func() {
		// Initialize context to match the configurations that the
		// specs are using.
		ctx, err := gokzg4844.NewContext4096Insecure1337()
		if err != nil {
			panic(fmt.Sprintf("could not create context, err : %v", err))
		}
		gCryptoCtx = *ctx
		// Initialize the precompile return value
	})
}

// CryptoCtx returns a context object stores all of the necessary configurations
// to allow one to create and verify blob proofs.
// This function is expensive to run if the crypto context isn't initialized, so it is recommended to
// pre-initialize by calling InitializeCryptoCtx
func CryptoCtx() gokzg4844.Context {
	InitializeCryptoCtx()
	return gCryptoCtx
}

type BlobID uint64

type BlobIDs []BlobID

func GetBlobList(startId BlobID, count uint64) BlobIDs {
	blobList := make(BlobIDs, count)
	for i := uint64(0); i < count; i++ {
		blobList[i] = startId + BlobID(i)
	}
	return blobList
}

func GetBlobListByIndex(startIndex BlobID, endIndex BlobID) BlobIDs {
	count := uint64(0)
	if endIndex > startIndex {
		count = uint64(endIndex - startIndex + 1)
	} else {
		count = uint64(startIndex - endIndex + 1)
	}
	blobList := make(BlobIDs, count)
	if endIndex > startIndex {
		for i := uint64(0); i < count; i++ {
			blobList[i] = startIndex + BlobID(i)
		}
	} else {
		for i := uint64(0); i < count; i++ {
			blobList[i] = endIndex - BlobID(i)
		}
	}

	return blobList
}

// Blob transaction creator
type BlobTransactionCreator struct {
	To         *common.Address
	GasLimit   uint64
	GasFee     *big.Int
	GasTip     *big.Int
	DataGasFee *big.Int
	BlobID     BlobID
	BlobCount  uint64
	Value      *big.Int
	Data       []byte
	PrivateKey *ecdsa.PrivateKey
}

func (blobId BlobID) VerifyBlob(blob *typ.Blob) (bool, error) {
	if blob == nil {
		return false, errors.New("nil blob")
	}
	if blobId == 0 {
		// Blob zero is empty blob
		emptyFieldElem := [32]byte{}
		for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
			if !bytes.Equal(blob[chunkIdx*32:(chunkIdx+1)*32], emptyFieldElem[:]) {
				return false, nil
			}
		}
		return true, nil
	}

	// Check the blob against the deterministic data
	blobIdBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blobIdBytes, uint64(blobId))

	// First 32 bytes are the hash of the blob ID
	currentHashed := sha256.Sum256(blobIdBytes)

	for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
		var expectedFieldElem [32]byte
		copy(expectedFieldElem[:], currentHashed[:])

		// Check that no 32 bytes chunks are greater than the BLS modulus
		for i := 0; i < 32; i++ {
			// blobByteIdx := 32 - i - 1
			blobByteIdx := i
			if expectedFieldElem[blobByteIdx] < gokzg4844.BlsModulus[i] {
				// done with this field element
				break
			} else if expectedFieldElem[blobByteIdx] >= gokzg4844.BlsModulus[i] {
				if gokzg4844.BlsModulus[i] > 0 {
					// This chunk is greater than the modulus, and we can reduce it in this byte position
					expectedFieldElem[blobByteIdx] = gokzg4844.BlsModulus[i] - 1
					// done with this field element
					break
				} else {
					// This chunk is greater than the modulus, but we can't reduce it in this byte position, so we will try in the next byte position
					expectedFieldElem[blobByteIdx] = gokzg4844.BlsModulus[i]
				}
			}
		}

		if !bytes.Equal(blob[chunkIdx*32:(chunkIdx+1)*32], expectedFieldElem[:]) {
			return false, nil
		}

		// Hash the current hash
		currentHashed = sha256.Sum256(currentHashed[:])
	}
	return true, nil
}

func (blobId BlobID) FillBlob(blob *typ.Blob) error {
	if blob == nil {
		return errors.New("nil blob")
	}
	if blobId == 0 {
		// Blob zero is empty blob, so leave as is
		return nil
	}
	// Fill the blob with deterministic data
	blobIdBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blobIdBytes, uint64(blobId))

	// First 32 bytes are the hash of the blob ID
	currentHashed := sha256.Sum256(blobIdBytes)

	for chunkIdx := 0; chunkIdx < typ.FieldElementsPerBlob; chunkIdx++ {
		copy(blob[chunkIdx*32:(chunkIdx+1)*32], currentHashed[:])

		// Check that no 32 bytes chunks are greater than the BLS modulus
		for i := 0; i < 32; i++ {
			//blobByteIdx := ((chunkIdx + 1) * 32) - i - 1
			blobByteIdx := (chunkIdx * 32) + i
			if blob[blobByteIdx] < gokzg4844.BlsModulus[i] {
				// go to next chunk
				break
			} else if blob[blobByteIdx] >= gokzg4844.BlsModulus[i] {
				if gokzg4844.BlsModulus[i] > 0 {
					// This chunk is greater than the modulus, and we can reduce it in this byte position
					blob[blobByteIdx] = gokzg4844.BlsModulus[i] - 1
					// go to next chunk
					break
				} else {
					// This chunk is greater than the modulus, but we can't reduce it in this byte position, so we will try in the next byte position
					blob[blobByteIdx] = gokzg4844.BlsModulus[i]
				}
			}
		}

		// Hash the current hash
		currentHashed = sha256.Sum256(currentHashed[:])
	}

	return nil
}

func (blobId BlobID) GenerateBlob() (*typ.Blob, *typ.KZGCommitment, error) {
	blob := typ.Blob{}
	if err := blobId.FillBlob(&blob); err != nil {
		return nil, nil, errors.Wrap(err, "GenerateBlob (1)")
	}
	ctx_4844 := CryptoCtx()

	kzgCommitment, err := ctx_4844.BlobToKZGCommitment(gokzg4844.Blob(blob), 0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "GenerateBlob (2)")
	}

	typesKzgCommitment := typ.KZGCommitment(kzgCommitment)

	return &blob, &typesKzgCommitment, nil
}

func (blobId BlobID) GetVersionedHash(commitmentVersion byte) (common.Hash, error) {
	_, kzgCommitment, err := blobId.GenerateBlob()
	if err != nil {
		return common.Hash{}, errors.Wrap(err, "GetVersionedHash")
	}
	if kzgCommitment == nil {
		return common.Hash{}, errors.New("nil kzgCommitment")
	}
	sha256Hash := sha256.Sum256((*kzgCommitment)[:])
	versionedHash := common.BytesToHash(append([]byte{commitmentVersion}, sha256Hash[1:]...))
	return versionedHash, nil
}

func BlobDataGenerator(startBlobId BlobID, blobCount uint64) ([]common.Hash, *typ.BlobTxWrapData, error) {
	blobData := typ.BlobTxWrapData{
		Blobs:       make(typ.Blobs, blobCount),
		Commitments: make([]typ.KZGCommitment, blobCount),
	}
	for i := uint64(0); i < blobCount; i++ {
		if blob, kzgCommitment, err := (startBlobId + BlobID(i)).GenerateBlob(); err != nil {
			return nil, nil, err
		} else {
			blobData.Blobs[i] = *blob
			blobData.Commitments[i] = *kzgCommitment
		}
	}

	var hashes []common.Hash
	for i := 0; i < len(blobData.Commitments); i++ {
		hashes = append(hashes, blobData.Commitments[i].ComputeVersionedHash())
	}
	_, _, proofs, err := blobData.Blobs.ComputeCommitmentsAndProofs(CryptoCtx())
	if err != nil {
		return nil, nil, err
	}
	blobData.Proofs = proofs
	return hashes, &blobData, nil
}

func (tc *BlobTransactionCreator) GetSourceAddress() common.Address {
	if tc.PrivateKey == nil {
		return globals.VaultAccountAddress
	}
	return crypto.PubkeyToAddress(tc.PrivateKey.PublicKey)
}

func (tc *BlobTransactionCreator) MakeTransaction(nonce uint64) (typ.Transaction, error) {
	// Need tx wrap data that will pass blob verification
	hashes, blobData, err := BlobDataGenerator(tc.BlobID, tc.BlobCount)
	if err != nil {
		return nil, err
	}

	if tc.To == nil {
		return nil, errors.New("nil to address")
	}
	address := *tc.To

	// Chain ID
	chainID := new(big.Int).Set(globals.ChainID)

	gasFeeCap := tc.GasFee
	if gasFeeCap == nil {
		gasFeeCap = globals.GasPrice
	}

	gasTipCap := tc.GasTip
	if gasTipCap == nil {
		gasTipCap = globals.GasTipPrice
	}

	sbtx := &types.BlobTx{
		ChainID:    chainID,
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        tc.GasLimit,
		To:         address,
		Value:      tc.Value,
		Data:       tc.Data,
		AccessList: nil,
		BlobFeeCap: tc.DataGasFee,
		BlobHashes: hashes,
	}

	key := tc.PrivateKey
	if key == nil {
		key = globals.VaultKey
	}

	signedTx, err := types.SignNewTx(key, types.NewCancunSigner(globals.ChainID), sbtx)
	if err != nil {
		return nil, err
	}
	return &typ.TransactionWithBlobData{
		Tx:       signedTx,
		BlobData: blobData,
	}, nil
}