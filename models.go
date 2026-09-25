package main

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ข้อมูลผู้พัฒนา (คุณณราธร เหมือนเหลา)
type Student struct {
	Name      string `json:"name"`
	StudentID string `json:"student_id"`
	Service   string `json:"service"`
	Port      int    `json:"port"`
	Database  string `json:"database"`
}

// ตาราง orders ตามสเปกเอกสาร
type Order struct {
	ID              string      `gorm:"type:uuid;primaryKey" json:"id"`
	CustomerID      string      `gorm:"type:uuid;not null;index" json:"customer_id"`
	RestaurantID    string      `gorm:"type:uuid;not null;index" json:"restaurant_id"`
	DeliveryAddress string      `gorm:"type:text;not null" json:"delivery_address"`
	PaymentMethod   string      `gorm:"type:varchar(20);not null" json:"payment_method"`
	TotalPrice      float64     `gorm:"type:decimal(10,2);not null" json:"total_price"`
	Status          string      `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	CreatedAt       time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
	Items           []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE;" json:"items"`
}

// ตาราง order_items ตามสเปกเอกสาร
type OrderItem struct {
	ID         string    `gorm:"type:uuid;primaryKey" json:"id"`
	OrderID    string    `gorm:"type:uuid;not null;index" json:"order_id"`
	MenuItemID string    `gorm:"type:uuid;not null" json:"menu_item_id"`
	ItemName   string    `gorm:"type:varchar(255);not null" json:"item_name"`
	UnitPrice  float64   `gorm:"type:decimal(10,2);not null" json:"unit_price"`
	Qty        int       `gorm:"not null" json:"qty"`
	Note       string    `gorm:"type:varchar(255)" json:"note,omitempty"`
	Subtotal   float64   `gorm:"type:decimal(10,2);not null" json:"subtotal"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at,omitempty"`
}

// Hook อัตโนมัติ: สร้าง UUID ก่อนบันทึกลงตาราง Order
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// Hook อัตโนมัติ: สร้าง UUID ก่อนบันทึกลงตาราง OrderItem
func (item *OrderItem) BeforeCreate(tx *gorm.DB) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	return nil
}

// DTO สำหรับรับข้อมูลตอนสั่งซื้ออาหาร
type CreateOrderItemRequest struct {
	MenuItemID string  `json:"menu_item_id" binding:"required"`
	ItemName   string  `json:"item_name" binding:"required"`   // Snapshot ชื่อเมนู
	UnitPrice  float64 `json:"unit_price" binding:"required"`  // Snapshot ราคา
	Qty        int     `json:"qty" binding:"required,min=1,max=20"`
	Note       string  `json:"note"`
}

type CreateOrderRequest struct {
	CustomerID      string                   `json:"customer_id" binding:"required"`
	RestaurantID    string                   `json:"restaurant_id" binding:"required"`
	DeliveryAddress string                   `json:"delivery_address" binding:"required"`
	PaymentMethod   string                   `json:"payment_method" binding:"required"`
	Items           []CreateOrderItemRequest `json:"items" binding:"required,min=1"`
}

// DTO สำหรับอัปเดตสถานะออเดอร์
type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
