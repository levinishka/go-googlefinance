package manager

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/levinishka/go-googlefinance/internal/balancer"
	"github.com/levinishka/go-googlefinance/internal/cache"
)

const googleFinancePriceCommand string = `=GOOGLEFINANCE("%s", "price")`

type Config struct {
	CredentialsPath         string
	SpreadsheetId           string
	TtlInSec                int64
	BalancerNumberOfThreads int
}

type GoogleFinanceManager struct {
	service       *sheets.Service
	spreadsheetId string

	balancer *balancer.GoogleFinanceBalancer
	cache    *cache.Cache
}

func NewGoogleFinanceManager(managerConfig *Config) (*GoogleFinanceManager, error) {
	ctx := context.Background()

	credentials, err := os.ReadFile(managerConfig.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to read credentials: %v", err)
	}

	config, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to parse credentials: %v", err)
	}

	service, err := sheets.NewService(ctx, option.WithHTTPClient(config.Client(context.Background())))
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create Sheets service: %v", err)
	}

	balancer, err := balancer.NewGoogleFinanceBalancer(managerConfig.BalancerNumberOfThreads)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create balancer: %v", err)
	}

	cache, err := cache.NewCache(1e4, managerConfig.TtlInSec)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create cache: %v", err)
	}

	return &GoogleFinanceManager{
		service:       service,
		spreadsheetId: managerConfig.SpreadsheetId,
		balancer:      balancer,
		cache:         cache,
	}, nil
}

func (m *GoogleFinanceManager) ReadPrices(tickers []string) (map[string]float64, error) {
	res := make(map[string]float64, len(tickers))
	expiredTickers := make([]string, 0, len(tickers))

	for _, ticker := range tickers {
		if price, found := m.cache.Get(ticker); found {
			res[ticker] = price
		} else {
			expiredTickers = append(expiredTickers, ticker)
		}
	}

	if len(expiredTickers) == 0 {
		return res, nil
	}

	column := m.balancer.Acquire()
	defer m.balancer.Release(column)

	columnRange := fmt.Sprintf("price!%s1:%s%d", column, column, len(tickers))

	formulas := make([][]any, 0, len(expiredTickers))
	for _, ticker := range expiredTickers {
		formulas = append(formulas, []any{fmt.Sprintf(googleFinancePriceCommand, ticker)})
	}

	_, err := m.service.Spreadsheets.Values.Update(m.spreadsheetId, columnRange, &sheets.ValueRange{
		Values: formulas,
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return nil, fmt.Errorf("ReadPrices: unable to write formulas: %v", err)
	}

	resp, err := m.readPricesWithRetry(columnRange)
	if err != nil {
		return nil, fmt.Errorf("ReadPrices: unable to read formulas: %v", err)
	}

	for i, price := range resp.Values {
		priceFloat, err := strconv.ParseFloat(price[0].(string), 64)
		if err != nil {
			priceFloat = -1
		} else {
			m.cache.Set(expiredTickers[i], priceFloat)
		}
		res[expiredTickers[i]] = priceFloat
	}

	_, err = m.service.Spreadsheets.Values.Clear(m.spreadsheetId, columnRange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return nil, fmt.Errorf("ReadPrices: unable to clear formulas: %v", err)
	}

	return res, nil
}

func (m *GoogleFinanceManager) readPricesWithRetry(columnRange string) (*sheets.ValueRange, error) {
	numOfRetries := 3
	var err error
	for i := 0; i < numOfRetries; i++ {
		if resp, err := m.service.Spreadsheets.Values.Get(m.spreadsheetId, columnRange).Do(); err == nil && len(resp.Values) > 0 {
			return resp, err
		}

		if i == numOfRetries-1 {
			break
		}
		fmt.Printf("unsuccessful reading from sheet, retry %d, sleep for %d", i, i+1)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return nil, fmt.Errorf("readPricesWithRetry: unable to read formulas after %d retries: %v", numOfRetries, err)
}
