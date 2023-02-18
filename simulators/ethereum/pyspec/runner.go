package main

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// ------------------------------------------------------------------------//
// loadFixtureTests() yields every test recursively within a fixture.json  //
// file from the given 'root' path. It passes the tests to the func() 'fn' //
// yielded directly within fixtureRunner(), such that workers can start to //
// run the tests against each client.									   //
// ------------------------------------------------------------------------//
func loadFixtureTests(t *hivesim.T, root string, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// check file is actually a fixture	
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() { 
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, "withdrawals_balance_within_block.json") {
			return nil
		}

		// extract fixture.json tests into fixtureTest structs
		var fixtureTests map[string] fixtureTest
		if err := common.LoadJSON(path, &fixtureTests); err != nil {
			t.Logf("invalid test file: %v, unable to load json", err)
			return nil
		}
		
		// Only feed in one fixture 
		for name, fixture := range fixtureTests {
			tc := testcase{fixture: fixture, name: name, filepath: path}
			// t.Logf("----- transactions: %v", fixture.json.Blocks[0].Transactions)	
			if err := tc.validate(); err != nil {
				t.Errorf("test validation failed for %s: %v", tc.name, err)
				continue
			}
			fn(tc)
		}
		return nil
	})
}
func loadFixturePayloads(t *hivesim.T, root string, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// check file is actually a fixture	
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() { 
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, "withdrawals_balance_within_block.json") {
			return nil
		}

		// extract fixture.json tests into fixtureTest structs
		var fixtureTests map[string] fixtureTest
		if err := common.LoadJSON(path, &fixtureTests); err != nil {
			t.Logf("invalid test file: %v, unable to load json", err)
			return nil
		}
		
		// Only feed in one fixture 
		for name, fixture := range fixtureTests {
			tc := testcase{fixture: fixture, name: name, filepath: path}
			// t.Logf("----- transactions: %v", fixture.json.Blocks[0].Transactions)	
			if err := tc.validate(); err != nil {
				t.Errorf("test validation failed for %s: %v", tc.name, err)
				continue
			}
			fn(tc)
		}
		return nil
	})
}

// --------------------------------------------------------------------------//
// validate() returns an error if the fixture fork network is not supported. //
// --------------------------------------------------------------------------//
func (tc *testcase) validate() error {
	network := tc.fixture.json.Network
	if _, exist := ruleset[network]; !exist {
		return fmt.Errorf("network `%v` not defined in ruleset", network)
	}
	return nil
}

// run launches the client and runs the test case against it.
func (tc *testcase) run(t *hivesim.T) {
	// start := time.Now()

	// get paths for testcase root, including genesis.json & blockRLPs artefacts.
	// rootDir, genesisJson, blockRLPs, err := tc.createArtefacts()
	_, genesisJson, _, err := tc.createArtefacts()
	if err != nil {
		t.Fatal("can't prepare artefacts:", err)
	}

	// manually update testcase-specific hivesim parameters.
	env := hivesim.Params{
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
	}
	
	tc.updateEnv(env)

	// initialise a client files map. use structure ["/genesis.json": "rootDir/genesis.json"].
	clientFiles := map[string]string{
		"/genesis.json": genesisJson,
	}

	// start client (also creates an engine RPC client internally)
	genesis := getGenesis(&tc.fixture.json) //todo
	testContext := context.Background()

	engineAPI := hive_rpc.HiveRPCEngineStarter{
		ClientType: tc.clientType,
		EnginePort: globals.EnginePortHTTP,
		EthPort:    globals.EthPortHTTP,
		JWTSecret:  globals.DefaultJwtTokenSecretBytes,
	}
	engineClient, err := engineAPI.StartClient(
		t, 
		testContext, 
		genesis,
	    env,
		clientFiles,
	)
	if err != nil {
		t.Fatal("can't start client with engine api:", err)
	}

	hashes := []common.Hash{}
	for _, block := range tc.fixture.json.Blocks {
		hashes = append(hashes, block.BlockHeader.Hash)
	}
	fmt.Print("------------ %v", hashes)
	
	pb, err := engineClient.GetPayloadBodiesByHashV1(context.Background(), hashes)
	fmt.Print("------------ %v", pb )

	// poll client for loaded block information
	// t2 := time.Now()
	// genesisHash, genesisResponse, err := getBlock(client.RPC(), "0x0")
	// _, genesisResponse, err := getBlock(ethClient.RPC(), "0x0")
	// if err != nil {
		// t.Fatalf("can't get genesis: %v", err)
	// }
	// fmt.Print("genesisResponse: %v \n", genesisResponse)
	// fmt.Print("Transactions: %v \n", tc.fixture.json.Blocks[0].Transactions)
	// fmt.Print("Withdrawals: %v \n", tc.fixture.json.Blocks[0].Withdrawals)

	// feed in blocks with engine API


	// wantGenesis := tc.fixture.json.Genesis.Hash
	// if !bytes.Equal(wantGenesis[:], genesisHash) {
		// t.Errorf("genesis hash mismatch:\n  want 0x%x\n   got 0x%x", wantGenesis, genesisHash)
		// if diffs, err := compareGenesis(genesisResponse, tc.fixture.json.Genesis); err == nil {
			// t.Logf("Found differences: %v", diffs)
		// }
		// return
	// }

	// verify postconditions
	// t3 := time.Now()
	// lastHash, lastResponse, err := getBlock(client.RPC(), "latest")
	// if err != nil {
		// t.Fatal("can't get latest block:", err)
	// }
	// wantBest := tc.fixture.json.BestBlock
	// if !bytes.Equal(wantBest[:], lastHash) {
		// t.Errorf("last block hash mismatch:\n  want 0x%x\n   got 0x%x", wantBest, lastHash)
		// t.Log("block response:", lastResponse)
		// return
	// }
// 
	// end := time.Now()
	// t.Logf(`test timing:
 		//  artefacts    %v
 		//  startClient  %v
 		//  checkGenesis %v
 		//  checkLatest  %v`, t1.Sub(start), t2.Sub(t1), t3.Sub(t2), end.Sub(t3))
}

// updateEnv sets environment variables from the test
func (tc *testcase) updateEnv(env hivesim.Params) {
	// Environment variables for rules.
	rules := ruleset[tc.fixture.json.Network]
	for k, v := range rules {
		env[k] = fmt.Sprintf("%d", v)
	}
	// Possibly disable POW.
	if tc.fixture.json.SealEngine == "NoProof" {
		env["HIVE_SKIP_POW"] = "1"
	}
}

func getGenesis(test *fixtureJSON) (*core.Genesis){
	genesis := &core.Genesis{
		Nonce:      test.Genesis.Nonce.Uint64(),
		Timestamp:  test.Genesis.Timestamp.Uint64(),
		ExtraData:  test.Genesis.ExtraData,
		GasLimit:   test.Genesis.GasLimit,
		Difficulty: test.Genesis.Difficulty,
		Mixhash:    test.Genesis.MixHash,
		Coinbase:   test.Genesis.Coinbase,
		BaseFee:    test.Genesis.BaseFee,
		Alloc:      test.Pre,
	}
	return genesis
}

// -------------------------------------------------------------------------------------//
// createArtefacts(): creates the genesisBlockHeader & blockRLPs artefacts from      //
// a testcase within a fixture.json file. These are stored within the client container. //
// -------------------------------------------------------------------------------------//
func (tc *testcase) createArtefacts() (string, string, []string, error) {
	// generate a unique key for testcase, use this to create root/blockDir.
	key := fmt.Sprintf("%x", sha1.Sum([]byte(tc.filepath+tc.name)))
	rootDir := filepath.Join(tc.clientType, key)
	blockDir := filepath.Join(rootDir, "blocks")

	// create and give blockDir directory permissions 0700.
	if err := os.MkdirAll(blockDir, 0700); err != nil {
		return "", "", nil, err
	}

	// extract certain tc.fixture.json fields into a geth genesis struct.
	genesis := getGenesis(&tc.fixture.json) //todo


	// reformat extracted genesis data and add it to a seperate json file, in rootDir.
	genBytes, _ := json.Marshal(genesis)
	genesisFile := filepath.Join(rootDir, "genesis.json")
	if err := ioutil.WriteFile(genesisFile, genBytes, 0777); err != nil {
		return rootDir, "", nil, fmt.Errorf("failed writing genesis: %v", err)
	}

	// write each block rlp to "blockDir/0001.rlp", ..., "blockDir/0010.rlp" in binary form.
	var blockRLPs []string
	for i, block := range tc.fixture.json.Blocks {
		rlpData := common.FromHex(block.Rlp)
		fname := fmt.Sprintf("%s/%04d.rlp", blockDir, i+1)
		if err := ioutil.WriteFile(fname, rlpData, 0777); err != nil {
			return rootDir, genesisFile, blockRLPs, fmt.Errorf("failed writing block %d: %v", i, err)
		}
		blockRLPs= append(blockRLPs, fname)
	}

	return rootDir, genesisFile, blockRLPs, nil
}

// getBlock fetches a block from the client under test.
func getBlock(client *rpc.Client, arg string) (blockhash []byte, responseJSON string, err error) {
	blockData := make(map[string]interface{})
	if err := client.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
		// Make one more attempt
		fmt.Println("Client connect failed, making one more attempt...")
		if err = client.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
			return nil, "", err
		}
	}

	// Capture all response data.
	resp, _ := json.MarshalIndent(blockData, "", "  ")
	responseJSON = string(resp)

	hash, ok := blockData["hash"]
	if !ok {
		return nil, responseJSON, fmt.Errorf("no block hash found in response")
	}
	hexHash, ok := hash.(string)
	if !ok {
		return nil, responseJSON, fmt.Errorf("block hash in response is not a string: `%v`", hash)
	}
	return common.HexToHash(hexHash).Bytes(), responseJSON, nil
}

// compareGenesis is a helper utility to print out diffs in the genesis returned from the client,
// and print out the differences found. This is to avoid gigantic outputs where 40K tests all
// spit out all the fields.
func compareGenesis(have string, want blockHeader) (string, error) {
	var haveGenesis blockHeader
	if err := json.Unmarshal([]byte(have), &haveGenesis); err != nil {
		return "", err
	}
	output := ""
	cmp := func(have, want interface{}, name string) {
		if haveStr, wantStr := fmt.Sprintf("%v", have), fmt.Sprintf("%v", want); haveStr != wantStr {
			output += fmt.Sprintf("genesis.%v - have %v, want %v \n", name, haveStr, wantStr)
		}
	}
	// No need to output the hash difference -- it's already printed before entering here
	//cmp(haveGenesis.Hash, want.Hash, "hash")
	cmp(haveGenesis.MixHash, want.MixHash, "mixHash")
	cmp(haveGenesis.ParentHash, want.ParentHash, "parentHash")
	cmp(haveGenesis.ReceiptTrie, want.ReceiptTrie, "receiptsRoot")
	cmp(haveGenesis.TransactionsTrie, want.TransactionsTrie, "transactionsRoot")
	cmp(haveGenesis.UncleHash, want.UncleHash, "sha3Uncles")
	cmp(haveGenesis.Bloom, want.Bloom, "bloom")
	cmp(haveGenesis.Number, want.Number, "number")
	cmp(haveGenesis.Coinbase, want.Coinbase, "miner")
	cmp(haveGenesis.ExtraData, want.ExtraData, "extraData")
	cmp(haveGenesis.Difficulty, want.Difficulty, "difficulty")
	cmp(haveGenesis.Timestamp, want.Timestamp, "timestamp")
	cmp(haveGenesis.BaseFee, want.BaseFee, "baseFeePerGas")
	cmp(haveGenesis.GasLimit, want.GasLimit, "gasLimit")
	cmp(haveGenesis.GasUsed, want.GasUsed, "gasused")
	cmp(haveGenesis.WithdrawalsRoot, want.WithdrawalsRoot, "withdrawalsRoot")
	return output, nil
}