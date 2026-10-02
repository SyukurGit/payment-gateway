package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"paymentg/internal/database"
	"paymentg/internal/model"
)

func TestMatcher(t *testing.T) {
	testDir := "./test_data_matcher"
	os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	db, err := database.New(testDir)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}
	defer db.Close()

	// 1. Create test App
	app := &model.App{
		ID:            "app_test_1",
		Name:          "Toko Test",
		APIKey:        "ak_test123",
		WebhookURL:    "http://127.0.0.1:9999/webhook",
		WebhookSecret: "secret123",
		IsActive:      true,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
	if err := db.CreateApp(app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// 2. Create pending order for Rp 50.237
	order := &model.Order{
		ID:             "ord_test_1",
		AppID:          app.ID,
		ReferenceID:    "INV-001",
		OriginalAmount: 50000,
		UniqueCode:     237,
		TotalAmount:    50237,
		Status:         model.StatusPending,
		ExpiresAt:      time.Now().Add(15 * time.Minute).Format(time.RFC3339),
		CreatedAt:      time.Now().Format(time.RFC3339),
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}
	if err := db.CreateOrder(order); err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// 3. Test Matcher with mock transaction list
	webhook := NewWebhookService(db)
	matcher := NewMatcher(db, webhook)

	transactions := []model.ShopeeTransaction{
		{
			TransactionID: "tx_dummy_12345",
			Amount:        "50.237",
			Status:        1, // Success
			CreateTime:    time.Now().Unix(),
		},
		{
			TransactionID: "tx_dummy_67890",
			Amount:        "100.000",
			Status:        3, // Failed status, should be ignored
			CreateTime:    time.Now().Unix(),
		},
	}

	matched, err := matcher.MatchTransactions(transactions)
	if err != nil {
		t.Fatalf("MatchTransactions error: %v", err)
	}

	if matched != 1 {
		t.Errorf("Expected 1 match, got %d", matched)
	}

	// Verify order status is PAID in DB
	updatedOrder, err := db.GetOrder("ord_test_1")
	if err != nil {
		t.Fatalf("Failed to get order: %v", err)
	}

	if updatedOrder.Status != model.StatusPaid {
		t.Errorf("Expected status PAID, got %s", updatedOrder.Status)
	}
	if updatedOrder.ShopeeTxID != "tx_dummy_12345" {
		t.Errorf("Expected ShopeeTxID tx_dummy_12345, got %s", updatedOrder.ShopeeTxID)
	}

	// Verify transaction is marked processed (anti double-match)
	isProcessed, err := db.IsTransactionProcessed("tx_dummy_12345")
	if err != nil || !isProcessed {
		t.Errorf("Expected tx_dummy_12345 to be processed")
	}

	// Running matcher again on the same list should return 0 new matches
	matchedAgain, err := matcher.MatchTransactions(transactions)
	if err != nil {
		t.Fatalf("Second MatchTransactions error: %v", err)
	}
	if matchedAgain != 0 {
		t.Errorf("Expected 0 matches on repeat, got %d", matchedAgain)
	}
}

func TestUniqueCodeGeneration(t *testing.T) {
	testDir := "./test_data_unique"
	os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	db, err := database.New(testDir)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}
	defer db.Close()

	// Generate code for base amount 50000
	code1, err := GenerateUniqueCode(db, 50000)
	if err != nil {
		t.Fatalf("Failed to generate unique code: %v", err)
	}
	if code1 < 1 || code1 > 999 {
		t.Errorf("Unique code out of range: %d", code1)
	}

	app := &model.App{
		ID:            "app_1",
		Name:          "Test App",
		APIKey:        "ak_test_key",
		WebhookURL:    "http://localhost:8080/hook",
		WebhookSecret: "secret",
		IsActive:      true,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
	if err := db.CreateApp(app); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Reserve it by creating an order
	order := &model.Order{
		ID:             "ord_unique_1",
		AppID:          "app_1",
		OriginalAmount: 50000,
		UniqueCode:     code1,
		TotalAmount:    50000 + int64(code1),
		Status:         model.StatusPending,
		ExpiresAt:      time.Now().Add(15 * time.Minute).Format(time.RFC3339),
		CreatedAt:      time.Now().Format(time.RFC3339),
		UpdatedAt:      time.Now().Format(time.RFC3339),
	}
	if err := db.CreateOrder(order); err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Generate another code for the same base amount - must not collide
	for i := 0; i < 20; i++ {
		code2, err := GenerateUniqueCode(db, 50000)
		if err != nil {
			t.Fatalf("Failed to generate unique code: %v", err)
		}
		if code2 == code1 {
			t.Errorf("Collision detected! Got duplicate code %d", code2)
		}
	}

	fmt.Println("All unique code tests passed with no collisions!")
}
