package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/makiuchi-d/gozxing"
	zxingqr "github.com/makiuchi-d/gozxing/qrcode"
	"github.com/skip2/go-qrcode"

	"multipay/internal/database"
	"multipay/internal/model"
)

type QRISEngine struct {
	mu         sync.RWMutex
	db         *database.DB
	staticQRIS string
	httpClient *http.Client
}

func NewQRISEngine(db *database.DB, staticQRIS string) *QRISEngine {
	return &QRISEngine{
		db:         db,
		staticQRIS: staticQRIS,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (q *QRISEngine) SetStaticQRIS(s string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.staticQRIS = s
}

func (q *QRISEngine) GetStaticQRIS() string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.staticQRIS
}

// DecodeQRImage reads and decodes a QR code from a JPEG/PNG image file
func DecodeQRImage(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}

	reader := zxingqr.NewQRCodeReader()
	res, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}

	return res.GetText(), nil
}

func crc16ccitt(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = crc << 1
			}
		}
	}
	return crc
}

type tlv struct {
	tag   string
	value string
}

func (q *QRISEngine) GenerateDynamicQRIS(amount int64) (string, error) {
	q.mu.RLock()
	str := q.staticQRIS
	q.mu.RUnlock()

	if str == "" {
		// Fallback to standard Indonesian EMVCo QRIS template for sandbox & initial setup
		str = "00020101021126610014ID.LINKAJA.WWW0118936009143000000000021500000000000000051440014ID.CO.QRIS.WWW0215ID10200210000050303UME5204581253033605802ID5916MULTIPAY GATEWAY6007JAKARTA61051234062070703A016304"
	}

	var tags []tlv
	for len(str) >= 4 {
		tag := str[:2]
		lengthStr := str[2:4]
		length, err := strconv.Atoi(lengthStr)
		if err != nil || len(str) < 4+length {
			return "", fmt.Errorf("invalid QRIS TLV segment for tag %s", tag)
		}
		value := str[4 : 4+length]
		tags = append(tags, tlv{tag: tag, value: value})
		str = str[4+length:]
	}

	hasTag54 := false
	for i, t := range tags {
		if t.tag == "01" {
			tags[i].value = "12" // Point of Initiation: Dynamic
		} else if t.tag == "54" {
			tags[i].value = strconv.FormatInt(amount, 10)
			hasTag54 = true
		}
	}

	if !hasTag54 {
		tags = append(tags, tlv{tag: "54", value: strconv.FormatInt(amount, 10)})
	}

	filteredTags := make([]tlv, 0)
	for _, t := range tags {
		if t.tag != "63" {
			filteredTags = append(filteredTags, t)
		}
	}

	sort.SliceStable(filteredTags, func(i, j int) bool {
		return filteredTags[i].tag < filteredTags[j].tag
	})

	var sb strings.Builder
	for _, t := range filteredTags {
		sb.WriteString(t.tag)
		sb.WriteString(fmt.Sprintf("%02d", len(t.value)))
		sb.WriteString(t.value)
	}

	sb.WriteString("6304")
	payload := sb.String()
	crc := crc16ccitt([]byte(payload))
	return fmt.Sprintf("%s%04X", payload, crc), nil
}

func (q *QRISEngine) GenerateQRCodeFile(content, outputPath string) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	return qrcode.WriteFile(content, qrcode.Medium, 320, outputPath)
}

// FetchShopeeTransactions polls ShopeePay Partner transaction list
func (q *QRISEngine) FetchShopeeTransactions(token string) ([]model.ShopeeTransaction, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("shopee token is empty")
	}

	type requestData struct {
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

	now := time.Now().Unix()
	var reqBody requestData
	reqBody.Data.Metadata.Token = token
	reqBody.Data.Metadata.Language = "id"
	reqBody.Data.Metadata.Timezone = "Asia/Jakarta"
	reqBody.Data.PageSize = 20
	reqBody.Data.Filter.StartTime = now - 30*60
	reqBody.Data.Filter.EndTime = now + 60*60
	reqBody.Data.Filter.ServiceList = []int{1, 3}
	reqBody.Data.Sorter.Field = "createTime"
	reqBody.Data.Sorter.Order = "descend"

	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://shopeepay.shopee.co.id/merchant/v1/partner-web/get-transaction-list", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 16; Pixel 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Origin", "https://partner.shopee.co.id")
	req.Header.Set("Referer", "https://partner.shopee.co.id/")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp model.ShopeeTransactionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("shopee api error: code %d, msg: %s", apiResp.Code, apiResp.Msg)
	}

	return apiResp.Data.List, nil
}

func ParseIDRAmount(amountStr string) (int64, error) {
	clean := strings.TrimSpace(amountStr)
	clean = strings.Split(clean, ",")[0]
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, "Rp", "")
	clean = strings.TrimSpace(clean)
	var amount int64
	_, err := fmt.Sscanf(clean, "%d", &amount)
	return amount, err
}
