package main

import (
	"fmt"
	"log"
	"os"
	"time"

	finance "github.com/levinishka/go-googlefinance"
)

func main() {
	config := &finance.Config{
		CredentialsPath:         os.Getenv("GOOGLE_SHEETS_CREDENTIALS"),
		SpreadsheetId:           os.Getenv("GOOGLE_SHEETS_ID"),
		CacheSize:               1e4,
		CacheTtlInSec:           300,
		BalancerNumberOfThreads: 3,
	}
	googleFinanceClient, err := finance.NewGoogleFinanceClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	res, err := googleFinanceClient.ReadPrices([]string{"VTI", "VGT", "GE", "T"})
	if err != nil {
		log.Fatalf("Failed to read prices: %v", err)
	}
	fmt.Println(res)

	time.Sleep(2 * time.Second)

	assets, err := googleFinanceClient.ReadAssets([]string{"VTI", "VGT", "GE", "T", "NO_TICKER"})
	if err != nil {
		log.Fatalf("Failed to read assets: %v", err)
	}
	for ticker, info := range assets {
		fmt.Printf("%s: price=%.2f name=%q currency=%q\n", ticker, info.Price, info.Name, info.Currency)
	}
}
