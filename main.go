package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

var narathonStudent = Student{
	Name:      "นายณราธร เหมือนเหลา",
	StudentID: "67114540174",
	Service:   "Order Service",
	Port:      8083,
	Database:  "order_db",
}

func main() {
	// 1. เชื่อมต่อฐานข้อมูล PostgreSQL
	InitDB()

	// 2. สร้าง Gin Router
	r := gin.Default()

	// เปิดให้เรียกข้าม Origin ได้ (CORS)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// -------------------------------------------------------------------------
	// 3. API Routes
	// -------------------------------------------------------------------------

	// ข้อมูลประจำตัวของคุณณราธร เหมือนเหลา
	r.GET("/", studentHandler)
	r.GET("/narathon", studentHandler)

	// Health Check
	r.GET("/health", healthHandler)

	// Order Service Endpoints
	api := r.Group("/api/v1")
	{
		api.POST("/orders", createOrderHandler)
		api.GET("/orders", getOrdersHandler)
		api.GET("/orders/:id", getOrderByIDHandler)
		api.PATCH("/orders/:id/status", updateOrderStatusHandler)
		api.PUT("/orders/:id/status", updateOrderStatusHandler)
		api.POST("/orders/:id/cancel", cancelOrderHandler)
	}

	// รองรับ path แบบสั้นด้วย (/orders)
	r.POST("/orders", createOrderHandler)
	r.GET("/orders", getOrdersHandler)
	r.GET("/orders/:id", getOrderByIDHandler)
	r.PATCH("/orders/:id/status", updateOrderStatusHandler)
	r.PUT("/orders/:id/status", updateOrderStatusHandler)
	r.POST("/orders/:id/cancel", cancelOrderHandler)

	// 4. รันเซิร์ฟเวอร์ที่ Port 8083
	port := getEnv("PORT", "8083")
	log.Printf("🚀 Order Service กำลังทำงานที่พอร์ต :%s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ เซิร์ฟเวอร์หยุดทำงาน: %v", err)
	}
}

// Handler ข้อมูลผู้พัฒนา
func studentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Order Service (Food Ordering Microservices)",
		"author":  narathonStudent,
	})
}

// Handler ตรวจสอบสถานะ Server และ Database
func healthHandler(c *gin.Context) {
	dbStatus := "connected"
	if DB == nil {
		dbStatus = "disconnected"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"service":         "order-service",
		"database_status": dbStatus,
	})
}

// Handler สร้างออเดอร์ใหม่ (Create Order + Items Snapshot)
func createOrderHandler(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ฐานข้อมูล PostgreSQL ยังไม่ได้เชื่อมต่อ"})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ข้อมูลไม่ถูกต้อง: " + err.Error()})
		return
	}

	// ตรวจสอบ payment_method
	if req.PaymentMethod != "cash" && req.PaymentMethod != "promptpay" && req.PaymentMethod != "credit_card" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "payment_method ต้องเป็น 'cash', 'promptpay' หรือ 'credit_card' เท่านั้น",
		})
		return
	}

	// คำนวณราคา Snapshot และ Subtotal ของแต่ละเมนู
	var totalPrice float64
	var orderItems []OrderItem

	for _, item := range req.Items {
		if item.Qty < 1 || item.Qty > 20 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "จำนวนสินค้า (qty) ต้องอยู่ระหว่าง 1 ถึง 20",
			})
			return
		}

		subtotal := item.UnitPrice * float64(item.Qty)
		totalPrice += subtotal

		orderItems = append(orderItems, OrderItem{
			MenuItemID: item.MenuItemID,
			ItemName:   item.ItemName,  // Snapshot ชื่อเมนู ณ ตอนสั่ง
			UnitPrice:  item.UnitPrice, // Snapshot ราคา ณ ตอนสั่ง
			Qty:        item.Qty,
			Note:       item.Note,
			Subtotal:   subtotal,
		})
	}

	order := Order{
		CustomerID:      req.CustomerID,
		RestaurantID:    req.RestaurantID,
		DeliveryAddress: req.DeliveryAddress,
		PaymentMethod:   req.PaymentMethod,
		TotalPrice:      totalPrice,
		Status:          "pending", // สถานะเริ่มต้นเป็น pending
		Items:           orderItems,
	}

	// บันทึกลง PostgreSQL พร้อม OrderItems (GORM จะบันทึกแบบ Transaction ให้)
	if err := DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "เกิดข้อผิดพลาดในการบันทึกออเดอร์: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "สร้างออเดอร์เรียบร้อยแล้ว",
		"data":    order,
	})
}

// Handler ดึงรายการออเดอร์ทั้งหมด
func getOrdersHandler(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ฐานข้อมูล PostgreSQL ยังไม่ได้เชื่อมต่อ"})
		return
	}

	var orders []Order
	query := DB.Preload("Items").Order("created_at DESC")

	// กรองตาม customer_id ถ้าส่งมา
	if customerID := c.Query("customer_id"); customerID != "" {
		query = query.Where("customer_id = ?", customerID)
	}

	// กรองตาม restaurant_id ถ้าส่งมา
	if restaurantID := c.Query("restaurant_id"); restaurantID != "" {
		query = query.Where("restaurant_id = ?", restaurantID)
	}

	// กรองตาม status ถ้าส่งมา
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ไม่สามารถดึงข้อมูลได้: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(orders),
		"data":  orders,
	})
}

// Handler ดึงออเดอร์ตามรหัส ID
func getOrderByIDHandler(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ฐานข้อมูล PostgreSQL ยังไม่ได้เชื่อมต่อ"})
		return
	}

	id := c.Param("id")
	var order Order
	if err := DB.Preload("Items").First(&order, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบออเดอร์รหัสนี้"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// Handler อัปเดตสถานะออเดอร์
func updateOrderStatusHandler(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ฐานข้อมูล PostgreSQL ยังไม่ได้เชื่อมต่อ"})
		return
	}

	id := c.Param("id")
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "รูปแบบข้อมูลไม่ถูกต้อง"})
		return
	}

	// ตรวจสอบสถานะที่อนุญาต
	allowedStatuses := map[string]bool{
		"pending":   true,
		"confirmed": true,
		"cooking":   true,
		"ready":     true,
		"completed": true,
		"cancelled": true,
	}

	if !allowedStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "สถานะต้องเป็นหนึ่งในนี้: pending, confirmed, cooking, ready, completed, cancelled",
		})
		return
	}

	var order Order
	if err := DB.First(&order, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบออเดอร์รหัสนี้"})
		return
	}

	if err := DB.Model(&order).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ไม่สามารถเปลี่ยนสถานะได้: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "เปลี่ยนสถานะออเดอร์เรียบร้อยแล้ว",
		"id":      order.ID,
		"status":  req.Status,
	})
}

// Handler ขอยกเลิกออเดอร์
func cancelOrderHandler(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ฐานข้อมูล PostgreSQL ยังไม่ได้เชื่อมต่อ"})
		return
	}

	id := c.Param("id")
	var order Order
	if err := DB.First(&order, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบออเดอร์รหัสนี้"})
		return
	}

	if order.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ไม่สามารถยกเลิกออเดอร์ที่จัดส่งสำเร็จแล้วได้"})
		return
	}

	if err := DB.Model(&order).Update("status", "cancelled").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ไม่สามารถยกเลิกออเดอร์ได้: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ยกเลิกออเดอร์เรียบร้อยแล้ว",
		"id":      order.ID,
		"status":  "cancelled",
	})
}
