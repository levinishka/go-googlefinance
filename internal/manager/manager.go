package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/levinishka/go-googlefinance/internal/balancer"
	"github.com/levinishka/go-googlefinance/internal/cache"
)

const (
	googleFinancePriceCommand    = `=GOOGLEFINANCE("%s", "price")`
	googleFinanceNameCommand     = `=GOOGLEFINANCE("%s", "name")`
	googleFinanceCurrencyCommand = `=GOOGLEFINANCE("%s", "currency")`
	columnsPerAsset              = 3
	defaultSheetName             = "price"
	clearTimeout                 = 10 * time.Second
)

type AssetInfo struct {
	Price    float64
	Name     string
	Currency string
}

type Config struct {
	CredentialsPath         string
	SpreadsheetId           string
	SheetName               string
	CacheSize               int64
	CacheTtlInSec           int64
	BalancerNumberOfThreads int
	Logger                  *slog.Logger
}

type GoogleFinanceManager struct {
	service       *sheets.Service
	spreadsheetId string
	sheetName     string
	logger        *slog.Logger

	balancer *balancer.GoogleFinanceBalancer
	cache    *cache.Cache[AssetInfo]
}

func NewGoogleFinanceManager(managerConfig *Config) (*GoogleFinanceManager, error) {
	if managerConfig == nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: config must not be nil")
	}

	ctx := context.Background()

	credentials, err := os.ReadFile(managerConfig.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to read credentials: %w", err)
	}

	config, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to parse credentials: %w", err)
	}

	service, err := sheets.NewService(ctx, option.WithHTTPClient(config.Client(context.Background())))
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create Sheets service: %w", err)
	}

	balancer, err := balancer.NewGoogleFinanceBalancer(managerConfig.BalancerNumberOfThreads, columnsPerAsset)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create balancer: %w", err)
	}

	cache, err := cache.NewCache[AssetInfo](managerConfig.CacheSize, managerConfig.CacheTtlInSec)
	if err != nil {
		return nil, fmt.Errorf("NewGoogleFinanceManager: failed to create cache: %w", err)
	}

	sheetName := strings.TrimSpace(managerConfig.SheetName)
	if sheetName == "" {
		sheetName = defaultSheetName
	}

	return &GoogleFinanceManager{
		service:       service,
		spreadsheetId: managerConfig.SpreadsheetId,
		sheetName:     sheetName,
		logger:        managerConfig.Logger,
		balancer:      balancer,
		cache:         cache,
	}, nil
}

func (m *GoogleFinanceManager) ReadPrices(tickers []string) (map[string]float64, error) {
	return m.ReadPricesContext(context.Background(), tickers)
}

func (m *GoogleFinanceManager) ReadPricesContext(ctx context.Context, tickers []string) (map[string]float64, error) {
	assets, err := m.ReadAssetsContext(ctx, tickers)
	if err != nil {
		return nil, err
	}

	res := make(map[string]float64, len(assets))
	for ticker, info := range assets {
		res[ticker] = info.Price
	}
	return res, nil
}

func (m *GoogleFinanceManager) ReadAssets(tickers []string) (map[string]AssetInfo, error) {
	return m.ReadAssetsContext(context.Background(), tickers)
}

func (m *GoogleFinanceManager) ReadAssetsContext(ctx context.Context, tickers []string) (map[string]AssetInfo, error) {
	res := make(map[string]AssetInfo, len(tickers))
	expiredTickers := make([]string, 0, len(tickers))
	seen := make(map[string]struct{}, len(tickers))

	for _, ticker := range tickers {
		if err := validateTicker(ticker); err != nil {
			return nil, fmt.Errorf("ReadAssets: %w", err)
		}
		if info, found := m.cache.Get(ticker); found {
			res[ticker] = info
		} else if _, ok := seen[ticker]; !ok {
			seen[ticker] = struct{}{}
			expiredTickers = append(expiredTickers, ticker)
		}
	}

	if len(expiredTickers) == 0 {
		return res, nil
	}

	slot, err := m.balancer.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReadAssets: unable to acquire column slot: %w", err)
	}

	columnRange := formatSheetRange(m.sheetName, slot.Start, slot.End, len(expiredTickers))

	formulas := make([][]any, 0, len(expiredTickers))
	for _, ticker := range expiredTickers {
		clean := strings.TrimSpace(ticker)
		formulas = append(formulas, []any{
			fmt.Sprintf(googleFinancePriceCommand, clean),
			fmt.Sprintf(googleFinanceNameCommand, clean),
			fmt.Sprintf(googleFinanceCurrencyCommand, clean),
		})
	}

	_, err = m.service.Spreadsheets.Values.Update(m.spreadsheetId, columnRange, &sheets.ValueRange{
		Values: formulas,
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		m.balancer.Release(slot)
		return nil, fmt.Errorf("ReadAssets: unable to write formulas: %w", err)
	}

	defer func() {
		go m.clearAndRelease(columnRange, slot)
	}()

	resp, err := m.readValuesWithRetry(ctx, columnRange)
	if err != nil {
		return nil, fmt.Errorf("ReadAssets: unable to read formulas: %w", err)
	}

	for i, ticker := range expiredTickers {
		var row []any
		if i < len(resp.Values) {
			row = resp.Values[i]
		}
		info, valid := parseAssetRow(row)
		if valid {
			m.cache.Set(ticker, info)
		}
		res[ticker] = info
	}

	return res, nil
}

func (m *GoogleFinanceManager) clearAndRelease(columnRange string, slot balancer.ColumnSlot) {
	defer m.balancer.Release(slot)

	clearCtx, cancel := context.WithTimeout(context.Background(), clearTimeout)
	defer cancel()

	if _, err := m.service.Spreadsheets.Values.Clear(m.spreadsheetId, columnRange, &sheets.ClearValuesRequest{}).Context(clearCtx).Do(); err != nil && m.logger != nil {
		m.logger.Warn("unable to clear sheet range", "range", columnRange, "err", err)
	}
}

func (m *GoogleFinanceManager) readValuesWithRetry(ctx context.Context, columnRange string) (*sheets.ValueRange, error) {
	numOfRetries := 3
	var err error
	var resp *sheets.ValueRange
	for i := range numOfRetries {
		resp, err = m.service.Spreadsheets.Values.Get(m.spreadsheetId, columnRange).
			ValueRenderOption("UNFORMATTED_VALUE").
			Context(ctx).
			Do()
		if err == nil && len(resp.Values) > 0 && !hasLoadingValues(resp.Values) {
			return resp, nil
		}

		if i == numOfRetries-1 {
			break
		}
		if m.logger != nil {
			m.logger.Warn("unsuccessful reading from sheet, retrying", "attempt", i+1, "sleep_sec", i+1, "err", err)
		}

		timer := time.NewTimer(time.Duration(i+1) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("readValuesWithRetry: %w", ctx.Err())
		case <-timer.C:
		}
	}

	if err != nil {
		return nil, fmt.Errorf("readValuesWithRetry: unable to read formulas after %d retries: %w", numOfRetries, err)
	}
	return nil, fmt.Errorf("readValuesWithRetry: sheet values empty or still loading after %d retries", numOfRetries)
}

func validateTicker(ticker string) error {
	trimmed := strings.TrimSpace(ticker)
	if trimmed == "" {
		return fmt.Errorf("invalid ticker: must not be empty")
	}
	for _, r := range trimmed {
		if r == '"' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid ticker %q: contains disallowed character", ticker)
		}
	}
	return nil
}

func formatSheetRange(sheetName, startCol, endCol string, rows int) string {
	needsQuoting := false
	for _, r := range sheetName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			needsQuoting = true
			break
		}
	}
	if needsQuoting {
		escaped := strings.ReplaceAll(sheetName, "'", "''")
		return fmt.Sprintf("'%s'!%s1:%s%d", escaped, startCol, endCol, rows)
	}
	return fmt.Sprintf("%s!%s1:%s%d", sheetName, startCol, endCol, rows)
}

func hasLoadingValues(values [][]any) bool {
	for _, row := range values {
		for _, cell := range row {
			s := strings.ToLower(strings.TrimSpace(fmt.Sprint(cell)))
			if s == "loading..." || s == "#loading..." {
				return true
			}
		}
	}
	return false
}

func isSheetError(val string) bool {
	if val == "" {
		return true
	}
	if strings.HasPrefix(val, "#") {
		return true
	}
	return strings.EqualFold(val, "loading...")
}

func cellString(row []any, idx int) string {
	if idx >= len(row) || row[idx] == nil {
		return ""
	}
	if s, ok := row[idx].(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(row[idx]))
}

func parseAssetRow(row []any) (AssetInfo, bool) {
	price := -1.0
	if len(row) > 0 && row[0] != nil {
		if f, ok := row[0].(float64); ok {
			price = f
		} else {
			rawPrice := cellString(row, 0)
			if !isSheetError(rawPrice) {
				if parsed, err := strconv.ParseFloat(rawPrice, 64); err == nil {
					price = parsed
				}
			}
		}
	}

	rawName := cellString(row, 1)
	name := ""
	if !isSheetError(rawName) {
		name = rawName
	}

	rawCurrency := cellString(row, 2)
	currency := ""
	if !isSheetError(rawCurrency) {
		currency = rawCurrency
	}

	return AssetInfo{
		Price:    price,
		Name:     name,
		Currency: currency,
	}, price >= 0
}
