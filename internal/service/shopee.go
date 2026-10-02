package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"paymentg/internal/model"
)

type ShopeeClient struct {
	httpClient *http.Client
}

func NewShopeeClient() *ShopeeClient {
	return &ShopeeClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type ShopeeRequest struct {
	Data struct {
		Metadata struct {
			Token    string `json:"token"`
			Language string `json:"language"`
			Timezone string `json:"timezone"`
		} `json:"metadata"`
		PageSize int `json:"pageSize"`
		Filter   struct {
			StartTime   int64 `json:"startTime"`
			EndTime     int64 `json:"endTime"`
			ServiceList []int `json:"serviceList"`
		} `json:"filter"`
		Sorter struct {
			Field string `json:"field"`
			Order string `json:"order"`
		} `json:"sorter"`
		NextPosition string `json:"next_position"`
	} `json:"data"`
}

func (s *ShopeeClient) FetchTransactions(token string) ([]model.ShopeeTransaction, error) {
	reqData := ShopeeRequest{}
	reqData.Data.Metadata.Token = token
	reqData.Data.Metadata.Language = "id"
	reqData.Data.Metadata.Timezone = "Asia/Jakarta"
	reqData.Data.PageSize = 20
	now := time.Now().Unix()
	reqData.Data.Filter.StartTime = now - 30*60
	reqData.Data.Filter.EndTime = now + 60*60
	reqData.Data.Filter.ServiceList = []int{1, 3}
	reqData.Data.Sorter.Field = "createTime"
	reqData.Data.Sorter.Order = "descend"
	reqData.Data.NextPosition = ""

	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://shopeepay.shopee.co.id/merchant/v1/partner-web/get-transaction-list", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 16; Pixel 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Origin", "https://partner.shopee.co.id")
	req.Header.Set("Referer", "https://partner.shopee.co.id/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp model.ShopeeTransactionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("shopee api error: code %d", apiResp.Code)
	}

	return apiResp.Data.List, nil
}

func ParseAmount(amountStr string) (int64, error) {
	clean := strings.TrimSpace(amountStr)
	clean = strings.Split(clean, ",")[0] // strip decimal cents if any
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, "Rp", "")
	clean = strings.TrimSpace(clean)
	var amount int64
	_, err := fmt.Sscanf(clean, "%d", &amount)
	return amount, err
}
