package service

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"paymentg/internal/database"

	"github.com/makiuchi-d/gozxing"
	zxingqr "github.com/makiuchi-d/gozxing/qrcode"
	"github.com/skip2/go-qrcode"
)

// DecodeQRImage reads and decodes a QR code from a JPEG or PNG image file
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

type QRISService struct {
	staticQRIS string
}

func NewQRISService(qrisString string) *QRISService {
	return &QRISService{
		staticQRIS: qrisString,
	}
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

func (q *QRISService) Generate(amount int64) (string, error) {
	if q.staticQRIS == "" {
		return "", errors.New("static QRIS string is empty")
	}

	str := q.staticQRIS
	var tags []tlv

	for len(str) >= 4 {
		tag := str[:2]
		lengthStr := str[2:4]
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return "", fmt.Errorf("invalid length for tag %s: %v", tag, err)
		}
		if len(str) < 4+length {
			return "", fmt.Errorf("string too short for tag %s", tag)
		}
		value := str[4 : 4+length]
		tags = append(tags, tlv{tag: tag, value: value})
		str = str[4+length:]
	}

	hasTag54 := false
	for i, t := range tags {
		if t.tag == "01" {
			tags[i].value = "12"
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

func (q *QRISService) GenerateQRImage(amount int64, size int) ([]byte, error) {
	dyn, err := q.Generate(amount)
	if err != nil {
		return nil, err
	}
	return qrcode.Encode(dyn, qrcode.Medium, size)
}

func GenerateUniqueCode(db *database.DB, baseAmount int64) (int, error) {
	for i := 0; i < 50; i++ {
		code := rand.IntN(999) + 1
		total := baseAmount + int64(code)
		inUse, err := db.IsAmountInUse(total)
		if err != nil {
			return 0, err
		}
		if !inUse {
			return code, nil
		}
	}
	return 0, errors.New("no available unique codes")
}
