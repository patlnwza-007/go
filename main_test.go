package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const testSecret = "0b78533e885cb954453e18266037bc9a80edb4227e61f7fc3f4f8e1787ce0167"

func generateTestToken(userID, email, role string) string {
	claims := JWTClaims{
		Sub:   userID,
		Email: email,
		Role:  role,
		Jti:   "test-jti",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(testSecret))
	return tokenString
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Public
	r.GET("/health", healthHandler)
	r.GET("/narathon", studentHandler)

	// API v1
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
	return r
}

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", testSecret)
	os.Setenv("DB_PORT", "5434")

	// Init DB connection
	dsn := "host=localhost user=postgres password=postgres dbname=order_db port=5434 sslmode=disable TimeZone=Asia/Bangkok"
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("Notice: DB not available for test: %v\n", err)
	} else {
		_ = DB.AutoMigrate(&Order{}, &OrderItem{})
	}

	os.Exit(m.Run())
}

// Test 1: Verify Unauthorized access when token is missing
func TestUnauthorizedWithoutToken(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", w.Code)
	}

	var res ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if res.Success != false || res.Error != "UNAUTHORIZED" {
		t.Fatalf("Expected error code UNAUTHORIZED, got %s", res.Error)
	}
}

// Test 2: State Machine status transition logic
func TestStatusTransitions(t *testing.T) {
	// Valid transitions
	if !isValidTransition("pending", "confirmed") {
		t.Errorf("Expected pending -> confirmed to be valid")
	}
	if !isValidTransition("pending", "cancelled") {
		t.Errorf("Expected pending -> cancelled to be valid")
	}
	if !isValidTransition("confirmed", "cooking") {
		t.Errorf("Expected confirmed -> cooking to be valid")
	}
	if !isValidTransition("cooking", "ready") {
		t.Errorf("Expected cooking -> ready to be valid")
	}
	if !isValidTransition("ready", "completed") {
		t.Errorf("Expected ready -> completed to be valid")
	}

	// Invalid transitions (skipping steps)
	if isValidTransition("pending", "ready") {
		t.Errorf("Expected pending -> ready to be invalid (cannot skip steps)")
	}
	if isValidTransition("cooking", "completed") {
		t.Errorf("Expected cooking -> completed to be invalid")
	}
	if isValidTransition("cooking", "cancelled") {
		t.Errorf("Expected cooking -> cancelled to be invalid (only pending/confirmed can cancel)")
	}
	if isValidTransition("completed", "cooking") {
		t.Errorf("Expected completed -> cooking to be invalid")
	}
}

// Test 3: Restaurant Service Unavailable returns 503
func TestRestaurantServiceUnavailable(t *testing.T) {
	if DB == nil {
		t.Skip("PostgreSQL not connected, skipping integration test")
	}

	// Set URL to an unreachable port
	os.Setenv("RESTAURANT_SERVICE_URL", "http://127.0.0.1:59999")

	router := setupTestRouter()
	token := generateTestToken("11111111-1111-1111-1111-111111111111", "customer@test.com", "customer")

	body := `{
		"restaurant_id": "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22",
		"delivery_address": "Engineering Building",
		"payment_method": "promptpay",
		"items": [
			{
				"menu_item_id": "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33",
				"qty": 2
			}
		]
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503 RESTAURANT_SERVICE_UNAVAILABLE, got %d: %s", w.Code, w.Body.String())
	}

	var res ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Error != "RESTAURANT_SERVICE_UNAVAILABLE" {
		t.Fatalf("Expected error code RESTAURANT_SERVICE_UNAVAILABLE, got %s", res.Error)
	}
}

// Test 4: Full Order creation with Mock Restaurant Service and Ownership Check
func TestOrderCreationAndOwnership(t *testing.T) {
	if DB == nil {
		t.Skip("PostgreSQL not connected, skipping integration test")
	}

	// Create Mock Restaurant Service
	mockRestaurant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/menu-items/") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"id":            "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33",
					"restaurant_id": "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22",
					"name":          "ข้าวกะเพราหมูกรอบ",
					"price":         65.0,
					"available":     true,
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockRestaurant.Close()

	os.Setenv("RESTAURANT_SERVICE_URL", mockRestaurant.URL)

	router := setupTestRouter()
	user1ID := "11111111-1111-1111-1111-111111111111"
	user2ID := "22222222-2222-2222-2222-222222222222"

	tokenUser1 := generateTestToken(user1ID, "user1@test.com", "customer")
	tokenUser2 := generateTestToken(user2ID, "user2@test.com", "customer")
	tokenAdmin := generateTestToken("33333333-3333-3333-3333-333333333333", "admin@test.com", "admin")

	// 1. Create order as User 1
	body := `{
		"restaurant_id": "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22",
		"delivery_address": "Engineering Building Room 301",
		"payment_method": "promptpay",
		"items": [
			{
				"menu_item_id": "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33",
				"qty": 2,
				"note": "เผ็ดน้อย"
			}
		]
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenUser1)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var createRes struct {
		Success bool  `json:"success"`
		Data    Order `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createRes); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	order := createRes.Data
	if order.CustomerID != user1ID {
		t.Errorf("Expected customer_id to be %s (from token), got %s", user1ID, order.CustomerID)
	}
	if order.TotalPrice != 130.0 {
		t.Errorf("Expected total_price 130.0, got %f", order.TotalPrice)
	}

	// 2. User 1 views their own order -> 200 OK
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenUser1)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for owner, got %d", w.Code)
	}

	// 3. User 2 tries to view User 1's order -> 403 Forbidden (Ownership Check)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenUser2)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-owner, got %d", w.Code)
	}

	// 4. Admin views order -> 200 OK
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenAdmin)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for admin, got %d", w.Code)
	}

	// 5. Test invalid status transition: pending -> ready (skipping confirmed and cooking) -> 400 Bad Request
	statusBody := `{"status": "ready"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PATCH", "/api/v1/orders/"+order.ID+"/status", strings.NewReader(statusBody))
	req.Header.Set("Authorization", "Bearer "+tokenAdmin)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request when skipping steps, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Test valid cancellation when pending
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/orders/"+order.ID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+tokenUser1)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when cancelling pending order, got %d: %s", w.Code, w.Body.String())
	}
}
