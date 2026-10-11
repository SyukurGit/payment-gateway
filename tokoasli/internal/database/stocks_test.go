package database

import (
	"os"
	"testing"
	"tokoasli/internal/model"
)

func TestStocksHoldAndCommitIdempotency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tokoasli_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 1. Create product
	prod := &model.Product{
		ID:           "test_prod_1",
		Name:         "Netflix Premium",
		Category:     "Streaming",
		Price:        35000,
		DeliveryType: model.DeliveryTypePool,
		IsActive:     true,
	}
	if err := db.CreateProduct(prod); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// 2. Add 2 stocks
	lines := []string{"acc1@test.com|pass1", "acc2@test.com|pass2"}
	added, err := db.AddStocks(prod.ID, lines)
	if err != nil || added != 2 {
		t.Fatalf("AddStocks failed, expected 2, got %d (err: %v)", added, err)
	}

	avail, err := db.CountAvailableStock(prod.ID)
	if err != nil || avail != 2 {
		t.Fatalf("Expected 2 available stocks, got %d", avail)
	}

	// 3. Hold 1 stock for Order A
	stockA, err := db.HoldStock(prod.ID, "ORD-A", 15)
	if err != nil {
		t.Fatalf("HoldStock failed: %v", err)
	}
	if stockA.Content != "acc1@test.com|pass1" {
		t.Fatalf("Expected acc1, got %s", stockA.Content)
	}

	// Available stock should now be 1
	availAfterHold, _ := db.CountAvailableStock(prod.ID)
	if availAfterHold != 1 {
		t.Fatalf("Expected 1 available stock, got %d", availAfterHold)
	}

	// 4. Commit stock for Order A
	contentFirst, err := db.CommitStock("ORD-A", prod.ID, model.DeliveryTypePool, "")
	if err != nil {
		t.Fatalf("CommitStock failed: %v", err)
	}
	if contentFirst != "acc1@test.com|pass1" {
		t.Fatalf("Expected acc1, got %s", contentFirst)
	}

	// 5. Test IDEMPOTENCY: Commit again for Order A (simulating duplicate webhook / manual check race)
	contentSecond, err := db.CommitStock("ORD-A", prod.ID, model.DeliveryTypePool, "")
	if err != nil {
		t.Fatalf("Second CommitStock failed: %v", err)
	}
	if contentSecond != contentFirst {
		t.Fatalf("Expected identical content on second commit, got %s vs %s", contentSecond, contentFirst)
	}

	// Verify that available stock is STILL 1 (it must NOT consume the second stock!)
	availAfterDoubleCommit, _ := db.CountAvailableStock(prod.ID)
	if availAfterDoubleCommit != 1 {
		t.Fatalf("Double commit consumed extra stock! Expected 1 available stock remaining, got %d", availAfterDoubleCommit)
	}
}
