package manager

import "testing"

func TestNewGoogleFinanceManager_NilConfig(t *testing.T) {
	m, err := NewGoogleFinanceManager(nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
	if m != nil {
		t.Fatal("expected manager to be nil on error")
	}
}

func TestValidateTicker(t *testing.T) {
	valid := []string{"VTI", "NASDAQ:GOOG", "BRK.A", "BF-B", "NO_TICKER", " AAPL "}
	for _, ticker := range valid {
		if err := validateTicker(ticker); err != nil {
			t.Errorf("validateTicker(%q) unexpected error: %v", ticker, err)
		}
	}

	invalid := []string{"", "   ", `AAPL"`, `VTI", "name")`, "VTI\n=IMPORTDATA()", "VTI\x00"}
	for _, ticker := range invalid {
		if err := validateTicker(ticker); err == nil {
			t.Errorf("validateTicker(%q) expected error, got nil", ticker)
		}
	}
}

func TestFormatSheetRange(t *testing.T) {
	tests := []struct {
		sheetName string
		startCol  string
		endCol    string
		rows      int
		want      string
	}{
		{"price", "A", "C", 4, "price!A1:C4"},
		{"stock_prices", "D", "F", 2, "stock_prices!D1:F2"},
		{"My Sheet", "A", "C", 3, "'My Sheet'!A1:C3"},
		{"O'Brien", "G", "I", 1, "'O''Brien'!G1:I1"},
	}

	for _, tc := range tests {
		got := formatSheetRange(tc.sheetName, tc.startCol, tc.endCol, tc.rows)
		if got != tc.want {
			t.Errorf("formatSheetRange(%q, %q, %q, %d) = %q, want %q", tc.sheetName, tc.startCol, tc.endCol, tc.rows, got, tc.want)
		}
	}
}

func TestParseAssetRow(t *testing.T) {
	tests := []struct {
		name      string
		row       []any
		wantInfo  AssetInfo
		wantValid bool
	}{
		{
			name: "valid row with strings",
			row:  []any{"285.42", "Vanguard Total Stock Market ETF", "USD"},
			wantInfo: AssetInfo{
				Price:    285.42,
				Name:     "Vanguard Total Stock Market ETF",
				Currency: "USD",
			},
			wantValid: true,
		},
		{
			name: "valid row with unformatted float64 >= 1000",
			row:  []any{12345.67, "Berkshire Hathaway Inc.", "USD"},
			wantInfo: AssetInfo{
				Price:    12345.67,
				Name:     "Berkshire Hathaway Inc.",
				Currency: "USD",
			},
			wantValid: true,
		},
		{
			name: "invalid ticker with #N/A across all columns",
			row:  []any{"#N/A", "#N/A", "#N/A"},
			wantInfo: AssetInfo{
				Price:    -1,
				Name:     "",
				Currency: "",
			},
			wantValid: false,
		},
		{
			name: "partial short row missing currency",
			row:  []any{"100.5", "Some Asset"},
			wantInfo: AssetInfo{
				Price:    100.5,
				Name:     "Some Asset",
				Currency: "",
			},
			wantValid: true,
		},
		{
			name: "empty row",
			row:  nil,
			wantInfo: AssetInfo{
				Price:    -1,
				Name:     "",
				Currency: "",
			},
			wantValid: false,
		},
		{
			name: "sheet error tokens in name and currency",
			row:  []any{"50.0", "#ERROR!", "#VALUE!"},
			wantInfo: AssetInfo{
				Price:    50.0,
				Name:     "",
				Currency: "",
			},
			wantValid: true,
		},
		{
			name: "loading token in price",
			row:  []any{"Loading...", "Test Corp", "EUR"},
			wantInfo: AssetInfo{
				Price:    -1,
				Name:     "Test Corp",
				Currency: "EUR",
			},
			wantValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInfo, gotValid := parseAssetRow(tc.row)
			if gotValid != tc.wantValid {
				t.Errorf("parseAssetRow(%v) valid = %v, want %v", tc.row, gotValid, tc.wantValid)
			}
			if gotInfo != tc.wantInfo {
				t.Errorf("parseAssetRow(%v) info = %+v, want %+v", tc.row, gotInfo, tc.wantInfo)
			}
		})
	}
}

func TestHasLoadingValues(t *testing.T) {
	tests := []struct {
		name   string
		values [][]any
		want   bool
	}{
		{
			name:   "no loading values",
			values: [][]any{{"100.0", "Apple Inc.", "USD"}},
			want:   false,
		},
		{
			name:   "contains Loading...",
			values: [][]any{{"100.0", "Loading...", "USD"}},
			want:   true,
		},
		{
			name:   "contains #LOADING... with whitespace",
			values: [][]any{{" #LOADING... ", "Apple Inc.", "USD"}},
			want:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasLoadingValues(tc.values); got != tc.want {
				t.Errorf("hasLoadingValues(%v) = %v, want %v", tc.values, got, tc.want)
			}
		})
	}
}
