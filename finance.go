package finance

import (
	"github.com/levinishka/go-googlefinance/internal/manager"
)

type Config struct {
	CredentialsPath         string
	SpreadsheetId           string
	TtlInSec                int64
	BalancerNumberOfThreads int
}

type GoogleFinanceClient struct {
	mngr *manager.GoogleFinanceManager
}

func NewGoogleFinanceClient(clientConfig *Config) (*GoogleFinanceClient, error) {
	config := &manager.Config{
		CredentialsPath:         clientConfig.CredentialsPath,
		SpreadsheetId:           clientConfig.SpreadsheetId,
		TtlInSec:                clientConfig.TtlInSec,
		BalancerNumberOfThreads: clientConfig.BalancerNumberOfThreads,
	}
	mngr, err := manager.NewGoogleFinanceManager(config)
	if err != nil {
		return nil, err
	}
	return &GoogleFinanceClient{
		mngr: mngr,
	}, nil
}

func (client *GoogleFinanceClient) ReadPrices(tickers []string) (map[string]float64, error) {
	return client.mngr.ReadPrices(tickers)
}
