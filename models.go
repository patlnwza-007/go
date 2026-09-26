package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// Response Envelopes ตามมาตรฐานทั้ง 4 Services
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// โครงสร้าง JWT Claims ตรงตาม User Service
type JWTClaims struct {
	Sub   string `json:"sub"`   // user_id / customer_id
	Email string `json:"email"` // user email
	Role  string `json:"role"`  // customer, restaurant_owner, rider, admin
	Jti   string `json:"jti"`
	jwt.RegisteredClaims
}

// DTO สำหรับรับข้อมูลสั่งซื้ออาหารจาก Client
// (ห้ามรับ customer_id จาก body - ดึงจาก token)
// (ห้ามรับ unit_price/item_name จาก body - ดึงจาก Restaurant Service)
type CreateOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id" binding:"required"`
	Qty        int    `json:"qty" binding:"required,min=1,max=20"`
	Note       string `json:"note"`
}

type CreateOrderRequest struct {
	RestaurantID    string                   `json:"restaurant_id" binding:"required"`
	DeliveryAddress string                   `json:"delivery_address" binding:"required"`
	PaymentMethod   string                   `json:"payment_method" binding:"required"`
	Items           []CreateOrderItemRequest `json:"items" binding:"required,min=1"`
}

// DTO สำหรับอัปเดตสถานะออเดอร์
type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// โมเดลข้อมูลจาก Restaurant Service (GET /api/v1/menu-items/{id})
type MenuItemResponse struct {
	ID           string  `json:"id"`
	RestaurantID string  `json:"restaurant_id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	Category     string  `json:"category"`
	ImageURL     string  `json:"image_url"`
	Available    bool    `json:"available"`
}
