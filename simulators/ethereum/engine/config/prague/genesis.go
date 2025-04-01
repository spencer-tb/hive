package prague

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

// ConfigGenesis configures the genesis block for the Prague fork.
func ConfigGenesis(genesis *core.Genesis, forkTimestamp uint64) error {
	if genesis.Config.ShanghaiTime == nil {
		return fmt.Errorf("prague fork requires shanghai fork")
	}

	if genesis.Config.CancunTime == nil {
		return fmt.Errorf("prague fork requires cancun fork")
	}
	genesis.Config.PragueTime = &forkTimestamp
	if *genesis.Config.ShanghaiTime > forkTimestamp {
		return fmt.Errorf("prague fork must be after shanghai fork")
	}
	if *genesis.Config.CancunTime > forkTimestamp {
		return fmt.Errorf("prague fork must be after cancun fork")
	}
	if genesis.Timestamp >= forkTimestamp {
		if genesis.BlobGasUsed == nil {
			genesis.BlobGasUsed = new(uint64)
		}
		if genesis.ExcessBlobGas == nil {
			genesis.ExcessBlobGas = new(uint64)
		}
	}

	// >>> Cancun system contracts <<<
	// Add bytecode pre deploy to the EIP-4788 address.
	genesis.Alloc[BEACON_ROOTS_ADDRESS] = core.GenesisAccount{
		Balance: common.Big0,
		Nonce:   1,
		Code:    common.Hex2Bytes("3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"),
	}


	// >>> Prague system contracts <<<
 
	// EIP-
	// Simple deposit generator, source: https://gist.github.com/lightclient/54abb2af2465d6969fa6d1920b9ad9d7
	var depositsGeneratorCode = common.FromHex("6080604052366103aa575f603067ffffffffffffffff811115610025576100246103ae565b5b6040519080825280601f01601f1916602001820160405280156100575781602001600182028036833780820191505090505b5090505f8054906101000a900460ff1660f81b815f8151811061007d5761007c6103db565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff191690815f1a9053505f602067ffffffffffffffff8111156100c7576100c66103ae565b5b6040519080825280601f01601f1916602001820160405280156100f95781602001600182028036833780820191505090505b5090505f8054906101000a900460ff1660f81b815f8151811061011f5761011e6103db565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff191690815f1a9053505f600867ffffffffffffffff811115610169576101686103ae565b5b6040519080825280601f01601f19166020018201604052801561019b5781602001600182028036833780820191505090505b5090505f8054906101000a900460ff1660f81b815f815181106101c1576101c06103db565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff191690815f1a9053505f606067ffffffffffffffff81111561020b5761020a6103ae565b5b6040519080825280601f01601f19166020018201604052801561023d5781602001600182028036833780820191505090505b5090505f8054906101000a900460ff1660f81b815f81518110610263576102626103db565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff191690815f1a9053505f600867ffffffffffffffff8111156102ad576102ac6103ae565b5b6040519080825280601f01601f1916602001820160405280156102df5781602001600182028036833780820191505090505b5090505f8054906101000a900460ff1660f81b815f81518110610305576103046103db565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff191690815f1a9053505f8081819054906101000a900460ff168092919061035090610441565b91906101000a81548160ff021916908360ff160217905550507f649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c585858585856040516103a09594939291906104d9565b60405180910390a1005b5f80fd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f60ff82169050919050565b5f61044b82610435565b915060ff820361045e5761045d610408565b5b600182019050919050565b5f81519050919050565b5f82825260208201905092915050565b8281835e5f83830152505050565b5f601f19601f8301169050919050565b5f6104ab82610469565b6104b58185610473565b93506104c5818560208601610483565b6104ce81610491565b840191505092915050565b5f60a0820190508181035f8301526104f181886104a1565b9050818103602083015261050581876104a1565b9050818103604083015261051981866104a1565b9050818103606083015261052d81856104a1565b9050818103608083015261054181846104a1565b9050969550505050505056fea26469706673582212208569967e58690162d7d6fe3513d07b393b4c15e70f41505cbbfd08f53eba739364736f6c63430008190033")
 
	genesis.Alloc[genesis.Config.DepositContractAddress] = core.GenesisAccount{
		Balance: common.Big0,
		Nonce:   1,
		Code:    depositsGeneratorCode,
	}

	return nil
}

// Configure specific test genesis accounts related to Prague functionality.
func ConfigTestAccounts(genesis *core.Genesis) error {
	// Add accounts that use the DATAHASH opcode
	datahashCode := []byte{
		0x5F, // PUSH0
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x01)
		0x01,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x02)
		0x02,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x03)
		0x03,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
	}

	for i := 0; i < DATAHASH_ADDRESS_COUNT; i++ {
		address := common.BigToAddress(big.NewInt(0).Add(DATAHASH_START_ADDRESS, big.NewInt(int64(i))))
		// check first if the address is already in the genesis
		if _, ok := genesis.Alloc[address]; ok {
			panic(fmt.Errorf("reused address %s during genesis configuration for prague", address.Hex()))
		}
		genesis.Alloc[address] = types.Account{
			Code:    datahashCode,
			Balance: common.Big0,
		}
	}

	return nil
}
