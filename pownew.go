package main


import (
	"context"
	// "crypto/rand"
	"fmt"
	"math/rand"
	"math/big"
	"sync"
	"time"
	"crypto/ecdsa"
	"strconv"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	"github.com/sirupsen/logrus"
)

var (
	infuraURL       = "https://eth-mainnet.g.alchemy.com/v2/FL6BaqcJywosW4svjdO4oje47zbP45Yq"
	privateKeystr   = "私钥私钥私钥"
	workerCount     = 10
	logger          = logrus.New()
	gasTipCap       = 1000000000 // 1 Gwei 小费gas
    gasFeeCap       = 256000000000 // 256 Gwei max gas
	gasPrice        = 100000000000
	gasLimit 		= uint64(25000) 
	toAddress 	 	= common.HexToAddress("0x0000000000000000000000000000000000000000")
)

func getNonceDate(nonce int64) string {
	return "data:application/json,{\"p\":\"ierc-20\",\"op\":\"mint\",\"tick\":\"electron\",\"amt\":\"1000\",\"nonce\":\"" + strconv.FormatInt(nonce, 10) + "\"}"
}

var (
	client     *ethclient.Client
	once       sync.Once
	initErr    error
	privateKey *ecdsa.PrivateKey
	fromAddress common.Address
	nonceAddr  uint64
	chainID    *big.Int
)

func initClient() {
	logger.Info(color.GreenString("Establishing connection with Ethereum client..."))
	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		logger.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	logger.Info(color.GreenString("Successfully connected to Ethereum client."))
	privateKey, err = crypto.HexToECDSA(privateKeystr)
	if err != nil {
		logger.Fatalf("Error in parsing private key: %v", err)
	}

	chainID, err = client.NetworkID(context.Background())
	if err != nil {
		logger.Fatalf("Failed to get chainID: %v", err)
	}
	logger.Infof(color.GreenString("Successfully connected to Ethereum network with Chain ID: %v"), chainID)


	publicKey := privateKey.Public()
    publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
    if !ok {
        logger.Fatal("error casting public key to ECDSA")
    }

    fromAddress = crypto.PubkeyToAddress(*publicKeyECDSA)

	nonceAddr, err = client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		logger.Fatal(err)
	}
}

func GetEthClient() (*ethclient.Client, error) {
	once.Do(initClient)
	return client, initErr
}


func mineWorker(ctx context.Context, wg *sync.WaitGroup, client *ethclient.Client, resultChan chan<- int64, errorChan chan<- error, hashCountChan chan<- int) {
	defer wg.Done()

	// var nonce *big.Int
	// var err error

	for {
		select {
		case <-ctx.Done():
			return
		default:
			nonce := time.Now().UnixNano() + int64(rand.Intn(20))
			data := []byte(getNonceDate(nonce))
			//创建交易
			tx := types.NewTx(&types.DynamicFeeTx{
				Nonce:     nonceAddr,
				GasTipCap: big.NewInt(int64(gasTipCap)),
				GasFeeCap: big.NewInt(int64(gasFeeCap)),
				Gas:       gasLimit,
				To:        &toAddress,
				Value:     big.NewInt(0),
				Data:      data,
			})
			// tx := types.NewTransaction(nonceAddr, toAddress, big.NewInt(0), gasLimit, big.NewInt(int64(gasPrice)), data)
			//创建签名者
			signer := types.NewLondonSigner(chainID)
			//对交易进行签名
			signTx, err := types.SignTx(tx, signer, privateKey)
			if err != nil {
				logger.Fatal(err)
			}

			// signTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
			if err != nil {
				logger.Fatal(err)
			}
			// logger.Info("hash not match:" + signTx.Hash().Hex()[0:7])
			// logger.Info(nonce)
			if signTx.Hash().Hex()[0:9] == "0x1840000" {
				logger.Info("hash match:" + signTx.Hash().Hex())
				clientx, err := ethclient.Dial(infuraURL)
				if err != nil {
					logger.Fatal(err)
				}
				err = clientx.SendTransaction(context.Background(), signTx)
				if err != nil {
					logger.Fatal(err)
				}
				logger.Info("tx sent:" + signTx.Hash().Hex())
				resultChan <- nonce
				return
			}
			
			hashCountChan <- 1

		}
	}
}

func main() {
	banner := `蝗虫启动`
	fmt.Println(banner)
	// flag.Parse()
	writer := uilive.New()

	writer.Start()
	defer writer.Stop()
	client, _ = GetEthClient()

	resultChan := make(chan int64)
	errorChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info(color.YellowString("Mining workers started..."))

	hashCountChan := make(chan int)
	totalHashCount := 0
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				hashesPerSecond := float64(totalHashCount) / 1000.0
				fmt.Fprintf(writer, "%s[%s] %s\n", color.BlueString("Mining"), timestamp, color.GreenString("Total hashes per second: %8.2f K/s", hashesPerSecond))
				totalHashCount = 0
			case count := <-hashCountChan:
				totalHashCount += count
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go mineWorker(ctx, &wg, client, resultChan, errorChan, hashCountChan)
	}

	select {
	case nonce := <-resultChan:
		ticker.Stop()
		cancel()
		wg.Wait()
		logger.Infof(color.GreenString("Successfully discovered a valid nonce: %d"), nonce)

	case err := <-errorChan:
		cancel()
		wg.Wait()
		logger.Fatalf("Mining operation failed due to an error: %v", err)
	}
	logger.Info(color.GreenString("Mining process successfully completed"))
}

