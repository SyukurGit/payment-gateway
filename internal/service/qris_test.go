package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestQRISConversion(t *testing.T) {
	staticQRIS := "00020101021126610016ID.CO.SHOPEE.WWW01189360091800238108140208238108140303UMI51440014ID.CO.QRIS.WWW0215ID10266074545470303UMI5204481453033605802ID5903A766013JAKARTA PUSAT61051035062070703A0163044CC7"

	svc := NewQRISService(staticQRIS)
	dynamicQRIS, err := svc.Generate(50237)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	t.Logf("Generated Dynamic QRIS: %s", dynamicQRIS)

	// Check Point of Initiation method is 12 (Dynamic)
	if !strings.Contains(dynamicQRIS, "010212") {
		t.Errorf("Expected dynamic tag 010212, got: %s", dynamicQRIS)
	}

	// Check Tag 54 (Amount: 50237 -> length 05)
	if !strings.Contains(dynamicQRIS, "540550237") {
		t.Errorf("Expected amount tag 540550237, got: %s", dynamicQRIS)
	}

	// Verify CRC16 is valid
	idx := strings.LastIndex(dynamicQRIS, "6304")
	if idx == -1 || len(dynamicQRIS) != idx+8 {
		t.Fatalf("Invalid CRC16 structure in: %s", dynamicQRIS)
	}

	payloadWithoutCRC := dynamicQRIS[:idx+4]
	expectedCRC := dynamicQRIS[idx+4:]
	calculatedCRC := fmt.Sprintf("%04X", crc16ccitt([]byte(payloadWithoutCRC)))

	if expectedCRC != calculatedCRC {
		t.Errorf("CRC mismatch! Expected %s, calculated %s", expectedCRC, calculatedCRC)
	}

	// Test QR image generation
	imgBytes, err := svc.GenerateQRImage(50237, 256)
	if err != nil {
		t.Fatalf("GenerateQRImage failed: %v", err)
	}
	if len(imgBytes) == 0 {
		t.Errorf("Generated QR image is empty")
	}
	t.Logf("Generated QR image size: %d bytes", len(imgBytes))
}
