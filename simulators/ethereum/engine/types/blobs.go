package types

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Blob Types

const (
	BlobCommitmentVersionKZG uint8 = 0x01
	FieldElementsPerBlob     int   = 4096
)

type KZGCommitment [48]byte

func (p KZGCommitment) MarshalText() ([]byte, error) {
	return []byte("0x" + hex.EncodeToString(p[:])), nil
}

func (p KZGCommitment) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *KZGCommitment) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("KZGCommitment", text, p[:])
}

// KZGToVersionedHash implements kzg_to_versioned_hash from EIP-4844
func KZGToVersionedHash(kzg gokzg4844.KZGCommitment) common.Hash {
	h := sha256.Sum256(kzg[:])
	h[0] = BlobCommitmentVersionKZG

	return h
}

func (c KZGCommitment) ComputeVersionedHash() common.Hash {
	return common.Hash(KZGToVersionedHash(gokzg4844.KZGCommitment(c)))
}

type KZGProof [48]byte

func (p KZGProof) MarshalText() ([]byte, error) {
	return []byte("0x" + hex.EncodeToString(p[:])), nil
}

func (p KZGProof) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *KZGProof) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("KZGProof", text, p[:])
}

type BLSFieldElement [32]byte

func (p BLSFieldElement) String() string {
	return "0x" + hex.EncodeToString(p[:])
}

func (p *BLSFieldElement) UnmarshalText(text []byte) error {
	return hexutil.UnmarshalFixedText("BLSFieldElement", text, p[:])
}

type Blob [FieldElementsPerBlob * 32]byte

func (blob *Blob) MarshalText() ([]byte, error) {
	out := make([]byte, 2+FieldElementsPerBlob*32*2)
	copy(out[:2], "0x")
	hex.Encode(out[2:], blob[:])

	return out, nil
}

func (blob *Blob) String() string {
	v, err := blob.MarshalText()
	if err != nil {
		return "<invalid-blob>"
	}
	return string(v)
}

func (blob *Blob) UnmarshalText(text []byte) error {
	if blob == nil {
		return errors.New("cannot decode text into nil Blob")
	}
	l := 2 + FieldElementsPerBlob*32*2
	if len(text) != l {
		return fmt.Errorf("expected %d characters but got %d", l, len(text))
	}
	if !(text[0] == '0' && text[1] == 'x') {
		return fmt.Errorf("expected '0x' prefix in Blob string")
	}
	if _, err := hex.Decode(blob[:], text[2:]); err != nil {
		return fmt.Errorf("blob is not formatted correctly: %v", err)
	}

	return nil
}

type BlobKzgs []KZGCommitment

type KZGProofs []KZGProof

type Blobs []Blob

// Return KZG commitments, versioned hashes and the proofs that correspond to these blobs
func (blobs Blobs) ComputeCommitmentsAndProofs(cryptoCtx gokzg4844.Context) (commitments []KZGCommitment, versionedHashes []common.Hash, proofs []KZGProof, err error) {
	commitments = make([]KZGCommitment, len(blobs))
	proofs = make([]KZGProof, len(blobs))
	versionedHashes = make([]common.Hash, len(blobs))

	for i, blob := range blobs {
		commitment, err := cryptoCtx.BlobToKZGCommitment(gokzg4844.Blob(blob), 1)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not convert blob to commitment: %v", err)
		}

		proof, err := cryptoCtx.ComputeBlobKZGProof(gokzg4844.Blob(blob), commitment, 1)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not compute proof for blob: %v", err)
		}
		commitments[i] = KZGCommitment(commitment)
		proofs[i] = KZGProof(proof)
		versionedHashes[i] = common.Hash(KZGToVersionedHash(gokzg4844.KZGCommitment(commitment)))
	}

	return commitments, versionedHashes, proofs, nil
}

type BlobTxWrapData struct {
	Blobs       Blobs
	Commitments BlobKzgs
	Proofs      KZGProofs
}

// BlobsBundle holds the blobs of an execution payload
type BlobsBundle struct {
	Commitments []KZGCommitment `json:"commitments" gencodec:"required"`
	Blobs       []Blob          `json:"blobs"       gencodec:"required"`
	Proofs      []KZGProof      `json:"proofs"      gencodec:"required"`
}

func (bb *BlobsBundle) VersionedHashes(commitmentVersion byte) (*[]common.Hash, error) {
	if bb == nil {
		return nil, errors.New("nil blob bundle")
	}
	if bb.Commitments == nil {
		return nil, errors.New("nil commitments")
	}
	versionedHashes := make([]common.Hash, len(bb.Commitments))
	for i, commitment := range bb.Commitments {
		sha256Hash := sha256.Sum256(commitment[:])
		versionedHashes[i] = common.BytesToHash(append([]byte{commitmentVersion}, sha256Hash[1:]...))
	}
	return &versionedHashes, nil
}
