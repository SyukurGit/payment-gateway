package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tokoasli/internal/model"
)

// CreateProduct inserts a new product
func (d *DB) CreateProduct(p *model.Product) error {
	now := time.Now().UTC().Format(time.RFC3339)
	p.CreatedAt = now
	p.UpdatedAt = now

	query := `INSERT INTO products (id, name, category, price, description, image_url, delivery_type, fixed_content, is_active, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	isActive := 1
	if !p.IsActive {
		isActive = 0
	}

	_, err := d.db.Exec(query, p.ID, p.Name, p.Category, p.Price, p.Description, p.ImageURL, p.DeliveryType, p.FixedContent, isActive, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert product: %w", err)
	}
	return nil
}

// GetProduct retrieves a product by ID with its current available stock count
func (d *DB) GetProduct(id string) (*model.Product, error) {
	query := `SELECT id, name, category, price, description, image_url, delivery_type, fixed_content, is_active, created_at, updated_at
			  FROM products WHERE id = ?`

	row := d.db.QueryRow(query, id)
	var p model.Product
	var isActiveInt int
	var fixedContent sql.NullString
	var desc sql.NullString
	var img sql.NullString

	err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &desc, &img, &p.DeliveryType, &fixedContent, &isActiveInt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("product not found")
		}
		return nil, fmt.Errorf("failed to query product: %w", err)
	}

	p.IsActive = isActiveInt == 1
	p.Description = desc.String
	p.ImageURL = img.String
	p.FixedContent = fixedContent.String

	// Compute stock
	p.StockCount, _ = d.computeProductStock(p.ID, p.DeliveryType)
	return &p, nil
}

// ListActiveProducts returns all active products with stock count for public catalog
func (d *DB) ListActiveProducts() ([]model.Product, error) {
	query := `SELECT id, name, category, price, description, image_url, delivery_type, fixed_content, is_active, created_at, updated_at
			  FROM products WHERE is_active = 1 ORDER BY created_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active products: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		var isActiveInt int
		var fixedContent sql.NullString
		var desc sql.NullString
		var img sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &desc, &img, &p.DeliveryType, &fixedContent, &isActiveInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsActive = isActiveInt == 1
		p.Description = desc.String
		p.ImageURL = img.String
		p.FixedContent = fixedContent.String
		p.StockCount, _ = d.computeProductStock(p.ID, p.DeliveryType)

		products = append(products, p)
	}

	return products, nil
}

// ListAllProducts returns all products (including inactive) for admin dashboard
func (d *DB) ListAllProducts() ([]model.Product, error) {
	query := `SELECT id, name, category, price, description, image_url, delivery_type, fixed_content, is_active, created_at, updated_at
			  FROM products ORDER BY created_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list all products: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		var isActiveInt int
		var fixedContent sql.NullString
		var desc sql.NullString
		var img sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &desc, &img, &p.DeliveryType, &fixedContent, &isActiveInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsActive = isActiveInt == 1
		p.Description = desc.String
		p.ImageURL = img.String
		p.FixedContent = fixedContent.String
		p.StockCount, _ = d.computeProductStock(p.ID, p.DeliveryType)

		products = append(products, p)
	}

	return products, nil
}

// UpdateProduct updates product details
func (d *DB) UpdateProduct(p *model.Product) error {
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	isActive := 1
	if !p.IsActive {
		isActive = 0
	}

	query := `UPDATE products SET name = ?, category = ?, price = ?, description = ?, image_url = ?, delivery_type = ?, fixed_content = ?, is_active = ?, updated_at = ?
			  WHERE id = ?`

	res, err := d.db.Exec(query, p.Name, p.Category, p.Price, p.Description, p.ImageURL, p.DeliveryType, p.FixedContent, isActive, p.UpdatedAt, p.ID)
	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("product not found")
	}
	return nil
}

// DeleteProduct attempts to delete a product. If it has orders, it deactivates it instead (soft-archive) to prevent foreign key errors.
func (d *DB) DeleteProduct(id string) error {
	// Check if this product has existing orders
	var orderCount int
	_ = d.db.QueryRow("SELECT COUNT(*) FROM store_orders WHERE product_id = ?", id).Scan(&orderCount)
	if orderCount > 0 {
		// Soft archive by deactivating product so order history is preserved
		_, err := d.db.Exec("UPDATE products SET is_active = 0, updated_at = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339), id)
		if err != nil {
			return fmt.Errorf("failed to archive product with order history: %w", err)
		}
		return nil
	}

	res, err := d.db.Exec("DELETE FROM products WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("product not found")
	}
	return nil
}

func (d *DB) computeProductStock(productID, deliveryType string) (int, error) {
	if deliveryType == model.DeliveryTypeFixed {
		return 999, nil // Unlimited stock for fixed content
	}

	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM product_stocks WHERE product_id = ? AND is_used = 0", productID).Scan(&count)
	return count, err
}
