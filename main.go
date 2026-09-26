package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var narathonStudent = Student{
	Name:      "นายณราธร เหมือนเหลา",
	StudentID: "67114540174",
	Service:   "Order Service",
	Port:      8083,
	Database:  "order_db",
}

// -----------------------------------------------------------------------------
// Helper สำหรับส่ง Response Envelope ตามมาตรฐานทั้ง 4 Services
// -----------------------------------------------------------------------------

func respondSuccess(c *gin.Context, code int, message string, data any) {
	c.JSON(code, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func respondError(c *gin.Context, code int, errCode string, message string, details any) {
	c.JSON(code, ErrorResponse{
		Success: false,
		Error:   errCode,
		Message: message,
		Details: details,
	})
}

// -----------------------------------------------------------------------------
// JWT Authentication Middleware
// -----------------------------------------------------------------------------

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "ไม่พบ Authorization header ในคำขอ", nil)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "รูปแบบ Authorization header ต้องเป็น 'Bearer <token>'", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]
		jwtSecret := []byte(getEnv("JWT_SECRET", "0b78533e885cb954453e18266037bc9a80edb4227e61f7fc3f4f8e1787ce0167"))

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token ไม่ถูกต้องหรือหมดอายุ", err.Error())
			c.Abort()
			return
		}

		// บันทึกข้อมูลผู้ใช้ลง context เพื่อนำไปใช้ใน handlers ต่อไป
		c.Set("userID", claims.Sub)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)

		c.Next()
	}
}

// -----------------------------------------------------------------------------
// Inter-service Communication: เรียก Restaurant Service
// -----------------------------------------------------------------------------

var (
	ErrRestaurantUnavailable = errors.New("restaurant service unavailable")
	ErrRestaurantNotFound    = errors.New("restaurant not found")
	ErrMenuItemNotFound      = errors.New("menu item not found")
)

// ดึงข้อมูลเมนูอาหาร (GET /api/v1/menu-items/{id})
func fetchMenuItem(menuItemID string) (*MenuItemResponse, error) {
	baseURL := strings.TrimRight(getEnv("RESTAURANT_SERVICE_URL", "http://restaurant-service:8082"), "/")
	url := fmt.Sprintf("%s/api/v1/menu-items/%s", baseURL, menuItemID)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("⚠️ ไม่สามารถติดต่อ Restaurant Service ที่ %s: %v", url, err)
		return nil, ErrRestaurantUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMenuItemNotFound
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ Restaurant Service ตอบกลับสถานะ %d จาก URL %s", resp.StatusCode, url)
		return nil, ErrRestaurantUnavailable
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Success bool              `json:"success"`
		Data    *MenuItemResponse `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &envelope); err == nil && envelope.Data != nil && envelope.Data.Name != "" {
		return envelope.Data, nil
	}

	var direct MenuItemResponse
	if err := json.Unmarshal(bodyBytes, &direct); err == nil && direct.Name != "" {
		return &direct, nil
	}

	return nil, errors.New("invalid menu item response format")
}

// ดึงข้อมูลร้านอาหาร (GET /api/v1/restaurants/{id}) เพื่อตรวจสอบ owner_id
func fetchRestaurant(restaurantID string) (*RestaurantResponse, error) {
	baseURL := strings.TrimRight(getEnv("RESTAURANT_SERVICE_URL", "http://restaurant-service:8082"), "/")
	url := fmt.Sprintf("%s/api/v1/restaurants/%s", baseURL, restaurantID)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("⚠️ ไม่สามารถติดต่อ Restaurant Service ที่ %s: %v", url, err)
		return nil, ErrRestaurantUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrRestaurantNotFound
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ Restaurant Service ตอบกลับสถานะ %d จาก URL %s", resp.StatusCode, url)
		return nil, ErrRestaurantUnavailable
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Success bool                `json:"success"`
		Data    *RestaurantResponse `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &envelope); err == nil && envelope.Data != nil && envelope.Data.OwnerID != "" {
		return envelope.Data, nil
	}

	var direct RestaurantResponse
	if err := json.Unmarshal(bodyBytes, &direct); err == nil && direct.OwnerID != "" {
		return &direct, nil
	}

	return nil, errors.New("invalid restaurant response format")
}

// -----------------------------------------------------------------------------
// State Machine: ตรวจสอบลำดับขั้นตอนสถานะออเดอร์
// pending → confirmed → cooking → ready → completed
// -----------------------------------------------------------------------------

func isValidTransition(current, next string) bool {
	switch current {
	case "pending":
		return next == "confirmed" || next == "cancelled"
	case "confirmed":
		return next == "cooking" || next == "cancelled"
	case "cooking":
		return next == "ready"
	case "ready":
		return next == "completed"
	default:
		return false
	}
}

// -----------------------------------------------------------------------------
// Main Function
// -----------------------------------------------------------------------------

func main() {
	// 1. เชื่อมต่อฐานข้อมูล PostgreSQL
	InitDB()

	// 2. สร้าง Gin Router
	r := gin.Default()

	// เปิดให้เรียกข้าม Origin ได้ (CORS)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Public Routes
	r.GET("/health", healthHandler)
	r.GET("/healthz", healthHandler)
	r.GET("/narathon", studentHandler)
	r.GET("/api/v1/narathon", studentHandler)
	r.GET("/api/v1/health", healthHandler)
	r.GET("/api/v1/healthz", healthHandler)

	// API v1 Group (ลบเส้นทางที่ไม่มี /api/v1 ออกทั้งหมด)
	api := r.Group("/api/v1")
	{
		authorized := api.Group("")
		authorized.Use(authMiddleware())
		{
			authorized.POST("/orders", createOrderHandler)
			authorized.GET("/orders", getOrdersHandler)
			authorized.GET("/orders/:id", getOrderByIDHandler)
			authorized.GET("/orders/:id/status", getOrderStatusHandler)
			authorized.GET("/customers/:id/orders", getCustomerOrdersHandler)
			authorized.PATCH("/orders/:id/status", updateOrderStatusHandler)
			authorized.PUT("/orders/:id/status", updateOrderStatusHandler)
			authorized.POST("/orders/:id/cancel", cancelOrderHandler)
		}
	}

	// สตาร์ทที่ Port 8083
	port := getEnv("PORT", "8083")
	log.Printf("🚀 Order Service กำลังทำงานที่พอร์ต :%s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ เซิร์ฟเวอร์หยุดทำงาน: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func studentHandler(c *gin.Context) {
	respondSuccess(c, http.StatusOK, "Order Service (Food Ordering Microservices)", narathonStudent)
}

func healthHandler(c *gin.Context) {
	dbStatus := "connected"
	if DB == nil {
		dbStatus = "disconnected"
	}
	respondSuccess(c, http.StatusOK, "Service is healthy", gin.H{
		"service":         "order-service",
		"database_status": dbStatus,
	})
}

// Handler สั่งอาหาร
func createOrderHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	customerID := c.GetString("userID")
	if customerID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "ไม่พบรหัสผู้ใช้จาก Token", nil)
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "ข้อมูลที่ส่งมาไม่ถูกต้อง: "+err.Error(), nil)
		return
	}

	if req.PaymentMethod != "cash" && req.PaymentMethod != "promptpay" && req.PaymentMethod != "credit_card" {
		respondError(c, http.StatusBadRequest, "INVALID_PAYMENT_METHOD", "payment_method ต้องเป็น 'cash', 'promptpay' หรือ 'credit_card' เท่านั้น", nil)
		return
	}

	var totalPrice float64
	var orderItems []OrderItem

	for _, itemReq := range req.Items {
		if itemReq.Qty < 1 || itemReq.Qty > 20 {
			respondError(c, http.StatusBadRequest, "INVALID_QTY", "จำนวนสินค้า (qty) ต้องอยู่ระหว่าง 1 ถึง 20", nil)
			return
		}

		menuItem, err := fetchMenuItem(itemReq.MenuItemID)
		if err != nil {
			if errors.Is(err, ErrRestaurantUnavailable) {
				respondError(c, http.StatusServiceUnavailable, "RESTAURANT_SERVICE_UNAVAILABLE", "ไม่สามารถติดต่อ Restaurant Service เพื่อตรวจสอบราคาเมนูได้", nil)
				return
			}
			if errors.Is(err, ErrMenuItemNotFound) {
				respondError(c, http.StatusNotFound, "MENU_ITEM_NOT_FOUND", fmt.Sprintf("ไม่พบรายการเมนูรหัส: %s", itemReq.MenuItemID), nil)
				return
			}
			respondError(c, http.StatusInternalServerError, "MENU_VERIFICATION_FAILED", "เกิดข้อผิดพลาดในการตรวจสอบเมนูอาหาร: "+err.Error(), nil)
			return
		}

		if !menuItem.Available {
			respondError(c, http.StatusBadRequest, "MENU_ITEM_UNAVAILABLE", fmt.Sprintf("เมนู '%s' ปิดการขายชั่วคราว", menuItem.Name), nil)
			return
		}

		if menuItem.RestaurantID != "" && menuItem.RestaurantID != req.RestaurantID {
			respondError(c, http.StatusBadRequest, "INVALID_RESTAURANT_ITEM", fmt.Sprintf("เมนู '%s' ไม่ได้เป็นของร้านที่เลือก", menuItem.Name), nil)
			return
		}

		subtotal := menuItem.Price * float64(itemReq.Qty)
		totalPrice += subtotal

		orderItems = append(orderItems, OrderItem{
			MenuItemID: itemReq.MenuItemID,
			ItemName:   menuItem.Name,
			UnitPrice:  menuItem.Price,
			Qty:        itemReq.Qty,
			Note:       itemReq.Note,
			Subtotal:   subtotal,
		})
	}

	order := Order{
		CustomerID:      customerID,
		RestaurantID:    req.RestaurantID,
		DeliveryAddress: req.DeliveryAddress,
		PaymentMethod:   req.PaymentMethod,
		TotalPrice:      totalPrice,
		Status:          "pending",
		Items:           orderItems,
	}

	if err := DB.Create(&order).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DATABASE_ERROR", "เกิดข้อผิดพลาดในการบันทึกออเดอร์: "+err.Error(), nil)
		return
	}

	respondSuccess(c, http.StatusCreated, "สร้างออเดอร์เรียบร้อยแล้ว", order)
}

// Handler ดึงรายการออเดอร์ทั้งหมด (Ownership Check ทั้ง Customer และ Restaurant Owner)
func getOrdersHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	currentUserID := c.GetString("userID")
	currentUserRole := c.GetString("userRole")

	var orders []Order
	query := DB.Preload("Items").Order("created_at DESC")

	if currentUserRole == "customer" {
		query = query.Where("customer_id = ?", currentUserID)
	} else if currentUserRole == "restaurant_owner" {
		restaurantID := c.Query("restaurant_id")
		if restaurantID == "" {
			respondError(c, http.StatusBadRequest, "MISSING_RESTAURANT_ID", "กรุณาระบุ restaurant_id สำหรับดูออเดอร์ของร้าน", nil)
			return
		}
		restaurant, err := fetchRestaurant(restaurantID)
		if err != nil {
			if errors.Is(err, ErrRestaurantUnavailable) {
				respondError(c, http.StatusServiceUnavailable, "RESTAURANT_SERVICE_UNAVAILABLE", "ไม่สามารถติดต่อ Restaurant Service เพื่อตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
				return
			}
			respondError(c, http.StatusForbidden, "FORBIDDEN", "ไม่สามารถตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
			return
		}
		if restaurant.OwnerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่ใช่เจ้าของร้านนี้ ไม่มีสิทธิ์ดูรายการออเดอร์", nil)
			return
		}
		query = query.Where("restaurant_id = ?", restaurantID)
	} else if customerID := c.Query("customer_id"); customerID != "" {
		query = query.Where("customer_id = ?", customerID)
	}

	if currentUserRole != "restaurant_owner" {
		if restaurantID := c.Query("restaurant_id"); restaurantID != "" {
			query = query.Where("restaurant_id = ?", restaurantID)
		}
	}

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&orders).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DATABASE_ERROR", "ไม่สามารถดึงข้อมูลออเดอร์ได้: "+err.Error(), nil)
		return
	}

	respondSuccess(c, http.StatusOK, "ดึงรายการออเดอร์สำเร็จ", orders)
}

// Handler ดึงออเดอร์ตาม ID (Ownership Check ทั้ง Customer และ Restaurant Owner)
func getOrderByIDHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	id := c.Param("id")
	var order Order
	if err := DB.Preload("Items").First(&order, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "ORDER_NOT_FOUND", "ไม่พบออเดอร์รหัสนี้", nil)
		return
	}

	currentUserID := c.GetString("userID")
	currentUserRole := c.GetString("userRole")

	if currentUserRole == "customer" {
		if order.CustomerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์เข้าถึงออเดอร์นี้", nil)
			return
		}
	} else if currentUserRole == "restaurant_owner" {
		restaurant, err := fetchRestaurant(order.RestaurantID)
		if err != nil {
			if errors.Is(err, ErrRestaurantUnavailable) {
				respondError(c, http.StatusServiceUnavailable, "RESTAURANT_SERVICE_UNAVAILABLE", "ไม่สามารถติดต่อ Restaurant Service เพื่อตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
				return
			}
			respondError(c, http.StatusForbidden, "FORBIDDEN", "ไม่สามารถตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
			return
		}
		if restaurant.OwnerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่ใช่เจ้าของร้านของออเดอร์นี้ ไม่มีสิทธิ์เข้าถึงข้อมูล", nil)
			return
		}
	}

	respondSuccess(c, http.StatusOK, "ดึงข้อมูลออเดอร์สำเร็จ", order)
}

// Handler ดึงสถานะออเดอร์ตาม ID (Ownership Check ทั้ง Customer และ Restaurant Owner)
func getOrderStatusHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	id := c.Param("id")
	var order Order
	if err := DB.Select("id", "customer_id", "restaurant_id", "status", "updated_at").First(&order, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "ORDER_NOT_FOUND", "ไม่พบออเดอร์รหัสนี้", nil)
		return
	}

	currentUserID := c.GetString("userID")
	currentUserRole := c.GetString("userRole")

	if currentUserRole == "customer" {
		if order.CustomerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์เข้าถึงสถานะออเดอร์นี้", nil)
			return
		}
	} else if currentUserRole == "restaurant_owner" {
		restaurant, err := fetchRestaurant(order.RestaurantID)
		if err != nil {
			if errors.Is(err, ErrRestaurantUnavailable) {
				respondError(c, http.StatusServiceUnavailable, "RESTAURANT_SERVICE_UNAVAILABLE", "ไม่สามารถติดต่อ Restaurant Service เพื่อตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
				return
			}
			respondError(c, http.StatusForbidden, "FORBIDDEN", "ไม่สามารถตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
			return
		}
		if restaurant.OwnerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่ใช่เจ้าของร้านของออเดอร์นี้ ไม่มีสิทธิ์เข้าถึงสถานะ", nil)
			return
		}
	}

	respondSuccess(c, http.StatusOK, "ดึงสถานะออเดอร์สำเร็จ", gin.H{
		"id":         order.ID,
		"status":     order.Status,
		"updated_at": order.UpdatedAt,
	})
}

// Handler ดึงออเดอร์ของลูกค้าเฉพาะราย
func getCustomerOrdersHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	targetCustomerID := c.Param("id")
	currentUserID := c.GetString("userID")
	currentUserRole := c.GetString("userRole")

	if currentUserRole == "customer" && currentUserID != targetCustomerID {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์เข้าถึงข้อมูลของลูกค้ารายอื่น", nil)
		return
	}

	var orders []Order
	if err := DB.Preload("Items").Where("customer_id = ?", targetCustomerID).Order("created_at DESC").Find(&orders).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DATABASE_ERROR", "ไม่สามารถดึงข้อมูลออเดอร์ได้: "+err.Error(), nil)
		return
	}

	respondSuccess(c, http.StatusOK, "ดึงรายการออเดอร์ของลูกค้าสำเร็จ", orders)
}

// Handler อัปเดตสถานะออเดอร์ (Ownership Check ฝั่ง Restaurant Owner)
func updateOrderStatusHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	currentUserRole := c.GetString("userRole")
	currentUserID := c.GetString("userID")

	if currentUserRole == "customer" {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "ลูกค้าไม่สามารถแก้ไขสถานะออเดอร์นี้ได้โดยตรง (ใช้ยกเลิกออเดอร์แทน)", nil)
		return
	}

	id := c.Param("id")
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง", nil)
		return
	}

	var order Order
	if err := DB.First(&order, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "ORDER_NOT_FOUND", "ไม่พบออเดอร์รหัสนี้", nil)
		return
	}

	// Ownership check: ถ้าเป็น restaurant_owner ต้องเป็นเจ้าของร้านจริง
	if currentUserRole == "restaurant_owner" {
		restaurant, err := fetchRestaurant(order.RestaurantID)
		if err != nil {
			if errors.Is(err, ErrRestaurantUnavailable) {
				respondError(c, http.StatusServiceUnavailable, "RESTAURANT_SERVICE_UNAVAILABLE", "ไม่สามารถติดต่อ Restaurant Service เพื่อตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
				return
			}
			respondError(c, http.StatusForbidden, "FORBIDDEN", "ไม่สามารถตรวจสอบสิทธิ์เจ้าของร้านได้", nil)
			return
		}
		if restaurant.OwnerID != currentUserID {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่ใช่เจ้าของร้านของออเดอร์นี้ ไม่มีสิทธิ์แก้ไขสถานะ", nil)
			return
		}
	}

	// ตรวจสอบลำดับการเปลี่ยนสถานะ ห้ามข้ามขั้น
	if !isValidTransition(order.Status, req.Status) {
		respondError(c, http.StatusBadRequest, "INVALID_STATUS_TRANSITION",
			fmt.Sprintf("ไม่สามารถเปลี่ยนสถานะจาก '%s' ไปเป็น '%s' ได้ตามลำดับวงจรชีวิตของออเดอร์", order.Status, req.Status), nil)
		return
	}

	if err := DB.Model(&order).Update("status", req.Status).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DATABASE_ERROR", "ไม่สามารถเปลี่ยนสถานะได้: "+err.Error(), nil)
		return
	}

	respondSuccess(c, http.StatusOK, "เปลี่ยนสถานะออเดอร์เรียบร้อยแล้ว", gin.H{
		"id":     order.ID,
		"status": req.Status,
	})
}

// Handler ขอยกเลิกออเดอร์
func cancelOrderHandler(c *gin.Context) {
	if DB == nil {
		respondError(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "ฐานข้อมูล PostgreSQL ยังไม่พร้อมให้บริการ", nil)
		return
	}

	id := c.Param("id")
	var order Order
	if err := DB.First(&order, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "ORDER_NOT_FOUND", "ไม่พบออเดอร์รหัสนี้", nil)
		return
	}

	currentUserID := c.GetString("userID")
	currentUserRole := c.GetString("userRole")
	if currentUserRole == "customer" && order.CustomerID != currentUserID {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์ยกเลิกออเดอร์นี้", nil)
		return
	}

	if order.Status != "pending" && order.Status != "confirmed" {
		respondError(c, http.StatusBadRequest, "ORDER_CANNOT_BE_CANCELLED",
			fmt.Sprintf("ไม่สามารถยกเลิกออเดอร์ได้ เนื่องจากสถานะปัจจุบันคือ '%s' (ยกเลิกได้เฉพาะ pending หรือ confirmed เท่านั้น)", order.Status), nil)
		return
	}

	if err := DB.Model(&order).Update("status", "cancelled").Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DATABASE_ERROR", "ไม่สามารถยกเลิกออเดอร์ได้: "+err.Error(), nil)
		return
	}

	respondSuccess(c, http.StatusOK, "ยกเลิกออเดอร์เรียบร้อยแล้ว", gin.H{
		"id":     order.ID,
		"status": "cancelled",
	})
}
