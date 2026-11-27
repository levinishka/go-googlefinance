package finance

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/levinishka/go-googlefinance/internal/manager"
)

type Config struct {
	CredentialsPath         string
	SpreadsheetId           string
	SheetName               string
	CacheSize               int64
	CacheTtlInSec           int64
	BalancerNumberOfThreads int
	Logger                  *slog.Logger
}

type AssetInfo struct {
	Price    float64
	Name     string
	Currency string
}

type GoogleFinanceClient struct {
	mngr *manager.GoogleFinanceManager
}

func NewGoogleFinanceClient(clientConfig *Config) (*GoogleFinanceClient, error) {
	if clientConfig == nil {
		return nil, fmt.Errorf("NewGoogleFinanceClient: config must not be nil")
	}

	config := &manager.Config{
		CredentialsPath:         clientConfig.CredentialsPath,
		SpreadsheetId:           clientConfig.SpreadsheetId,
		SheetName:               clientConfig.SheetName,
		CacheSize:               clientConfig.CacheSize,
		CacheTtlInSec:           clientConfig.CacheTtlInSec,
		BalancerNumberOfThreads: clientConfig.BalancerNumberOfThreads,
		Logger:                  clientConfig.Logger,
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
	return client.ReadPricesContext(context.Background(), tickers)
}

func (client *GoogleFinanceClient) ReadPricesContext(ctx context.Context, tickers []string) (map[string]float64, error) {
	return client.mngr.ReadPricesContext(ctx, tickers)
}

func (client *GoogleFinanceClient) ReadAssets(tickers []string) (map[string]AssetInfo, error) {
	return client.ReadAssetsContext(context.Background(), tickers)
}

func (client *GoogleFinanceClient) ReadAssetsContext(ctx context.Context, tickers []string) (map[string]AssetInfo, error) {
	raw, err := client.mngr.ReadAssetsContext(ctx, tickers)
	if err != nil {
		return nil, err
	}

	res := make(map[string]AssetInfo, len(raw))
	for ticker, info := range raw {
		res[ticker] = AssetInfo{
			Price:    info.Price,
			Name:     info.Name,
			Currency: info.Currency,
		}
	}
	return res, nil
}
