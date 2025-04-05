# go-googlefinance

A Golang package to fetch stock prices from [Google Finance](https://www.google.com/finance), by using the `=GOOGLEFINANCE()` [function](https://support.google.com/docs/answer/3093281) in Google Sheets.

## 🚀 Getting started

### 🔧 1. Prepare Your Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com)
2. [Create a new project](https://developers.google.com/workspace/guides/create-project)
3. [Enable Google Sheets API](https://support.google.com/googleapi/answer/6158841) for your project
4. [Create a service account](https://support.google.com/a/answer/7378726)
    - Follow **Step 4** on that page to generate the service account and download the JSON key
5. Save the key on the disk in JSON format


### 📄 2. Create a new Google Sheet and share access

1. Go to [Google Drive](https://drive.google.com/) and create a new Google Sheet
2. Copy the **email address** of your [service account](https://console.cloud.google.com/iam-admin/serviceaccounts)
3. [Share the sheet](https://support.google.com/a/users/answer/13309904) with that email and assign the **Editor** role
4. [Get your Google Sheet ID](https://developers.google.com/workspace/sheets/api/guides/concepts) - it's the string in the URL between `/d/` and `/edit`


### 🌍 3. Set environment variables

```bash
export GOOGLE_SHEETS_CREDENTIALS=/path/to/the/file/with/credentials
export GOOGLE_SHEETS_ID=YOUR_GOOGLE_SHEET_ID
```

### 📦 4. Installation

```bash
go get github.com/levinishka/go-googlefinance
```

## 🧪 Usage example

```golang
package main

import (
	"fmt"
	"log"
	"os"

	finance "github.com/levinishka/go-googlefinance"
)

func main() {
	config := &finance.Config{
		CredentialsPath:         os.Getenv("GOOGLE_SHEETS_CREDENTIALS"),
		SpreadsheetId:           os.Getenv("GOOGLE_SHEETS_ID"),
		TtlInSec:                300,
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
}
```
