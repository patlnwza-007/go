# ระบบสั่งอาหารและจัดการร้านอาหารออนไลน์ (Go Microservices)
## ส่วนที่ 3: Order Service (`order-service`)

* **ผู้รับผิดชอบ:** นายณราธร เหมือนเหลา (คนที่ 3)
* **รหัสนักศึกษา:** 67114540174
* **Service Port:** `8083` (Path ทั้งหมดขึ้นต้นด้วย `/api/v1/`)
* **Database:** PostgreSQL (`order_db`)
* **Docker Network:** `food_ordering_net` (Shared External Network)

---

## 📋 สรุป 9 จุดที่แก้ไขและปรับปรุงตามข้อกำหนดการเชื่อม 4 Services
1. ✅ **JWT Auth Middleware:** ตรวจสอบ Token ผ่าน `Authorization: Bearer <token>` ด้วย `JWT_SECRET` เดียวกันทั้งระบบ
2. ✅ **`customer_id` จาก Token:** ดึงจากฟิลด์ `sub` ใน Claims ไม่รับจาก Request Body
3. ✅ **Inter-service Call:** เรียก `GET /api/v1/menu-items/{id}` ที่ `restaurant-service:8082` เพื่อดึงชื่อและราคาจริงมาทำ Snapshot (ถ้าติดต่อไม่ได้ ตอบ 503 `RESTAURANT_SERVICE_UNAVAILABLE`)
4. ✅ **Ownership Check:** ลูกค้าดูและยกเลิกได้เฉพาะออเดอร์ของตนเอง (ยกเว้น `admin` หรือร้านค้า)
5. ✅ **Standard Response Envelope:** ทุก API คืนค่ารูปแบบเดียวกัน:
   - สำเร็จ: `{"success": true, "message": "...", "data": ...}`
   - ล้มเหลว: `{"success": false, "error": "CODE", "message": "...", "details": null}`
6. ✅ **Status Transition Check:** ตรวจสอบลำดับสถานะ `pending → confirmed → cooking → ready → completed` ห้ามข้ามขั้น
7. ✅ **Cancellation Rules:** ยกเลิกได้เฉพาะออเดอร์ที่สถานะเป็น `pending` หรือ `confirmed` เท่านั้น
8. ✅ **Clean Routes:** ตัด Path ที่ไม่มี prefix `/api/v1` ออกทั้งหมด
9. ✅ **Added Endpoints:** เพิ่ม `GET /api/v1/customers/:id/orders` และ `GET /api/v1/orders/:id/status`

---

## 🛠️ โครงสร้างไฟล์ในโปรเจกต์
```
go_lang_project/
├── main.go            # Gin Router, Middleware, Handlers และการเรียก Restaurant Service
├── models.go          # Structs ตาราง orders, order_items, JWT Claims, Response Envelopes
├── database.go        # ฟังก์ชันเชื่อมต่อ PostgreSQL (order_db) และ Auto-Migrate
├── schema.sql         # คำสั่ง SQL DDL สำหรับสร้างตาราง
├── Dockerfile         # Multi-stage Docker build สำหรับ order-service
├── docker-compose.yml # รันทั้ง order-db และ order-service บนเครือข่าย food_ordering_net
├── .env               # กำหนดค่าพอร์ต, การเชื่อมต่อ DB, JWT_SECRET, Service URLs
└── go.mod             # ไฟล์จัดการ Go Dependencies
```

---

## 🚀 วิธีการรันโปรเจกต์

### รันผ่าน Docker Compose (แนะนำสำหรับการรวมระบบ 4 Services)
```bash
# 1. ตรวจสอบ/สร้าง shared network ก่อน (รันครั้งเดียว)
docker network create food_ordering_net

# 2. สั่ง build และเปิดทั้ง database และ service
docker compose up -d --build
```

### หรือรันแบบ Local (สำหรับพัฒนาในเครื่อง)
```bash
# 1. เปิดเฉพาะ database ผ่าน docker
docker compose up -d order_db

# 2. รัน Go service บนเครื่อง
go run .
```

---

## 📡 รายการ API Endpoints (ทั้งหมดอยู่ภายใต้ `/api/v1`)

| Method | Endpoint | สิทธิ์ (Role) | คำอธิบาย |
|---|---|---|---|
| `GET` | `/health` | Public | ตรวจสอบสถานะ Service และ Database |
| `GET` | `/narathon` หรือ `/api/v1/narathon` | Public | ข้อมูลผู้พัฒนา (นายณราธร เหมือนเหลา) |
| `POST` | `/api/v1/orders` | Customer | สั่งซื้ออาหาร (ดึงราคาจริงจาก Restaurant Service) |
| `GET` | `/api/v1/orders` | Customer/Admin | ดูรายการออเดอร์ (Customer เห็นเฉพาะของตนเอง) |
| `GET` | `/api/v1/orders/:id` | Owner/Admin | ดูรายละเอียดออเดอร์ตามรหัส ID |
| `GET` | `/api/v1/orders/:id/status` | Owner/Admin | ดูเฉพาะสถานะของออเดอร์ |
| `GET` | `/api/v1/customers/:id/orders` | Owner/Admin | ดูประวัติออเดอร์ทั้งหมดของลูกค้ารายนั้น |
| `PATCH`| `/api/v1/orders/:id/status` | Restaurant/Admin/Rider | ปรับเปลี่ยนสถานะออเดอร์ (ห้ามข้ามขั้น) |
| `POST` | `/api/v1/orders/:id/cancel` | Customer/Admin | ขอยกเลิกออเดอร์ (เฉพาะ pending/confirmed) |

---

## 🧪 ตัวอย่างคำสั่งทดสอบ API ด้วย `curl`

### 1. ดูข้อมูลผู้พัฒนา (Public)
```bash
curl http://localhost:8083/narathon
```

---

### 2. สร้าง Token จำลองสำหรับทดสอบ (หรือรับจาก User Service Port 8081)
เมื่อทดสอบ สามารถใช้ Token ที่ได้จากการล็อกอินที่ User Service (`http://user-service:8081/api/v1/auth/login`) หรือใช้ Token ทดสอบที่ sign ด้วย `JWT_SECRET`

ตัวอย่างส่ง Header:
```bash
TOKEN="<JWT_TOKEN>"
```

---

### 3. สั่งซื้ออาหาร (`POST /api/v1/orders`)
> สังเกต: ไม่ต้องส่ง `customer_id` (ระบบอ่านจาก Token) และไม่ต้องส่ง `unit_price`/`item_name` (ระบบดึงจาก Restaurant Service)
```bash
curl -X POST http://localhost:8083/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "restaurant_id": "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22",
    "delivery_address": "อาคารเรียนรวม 2 ชั้น 3 มหาวิทยาลัย",
    "payment_method": "promptpay",
    "items": [
      {
        "menu_item_id": "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33",
        "qty": 2,
        "note": "เผ็ดน้อย ไม่ใส่ผักชี"
      }
    ]
  }'
```

---

### 4. ดึงสถานะออเดอร์ (`GET /api/v1/orders/:id/status`)
```bash
curl http://localhost:8083/api/v1/orders/<ORDER_ID>/status \
  -H "Authorization: Bearer $TOKEN"
```

---

### 5. อัปเดตสถานะออเดอร์ตามลำดับ (`PATCH /api/v1/orders/:id/status`)
ลำดับที่ถูกต้อง: `pending` ➔ `confirmed` ➔ `cooking` ➔ `ready` ➔ `completed`
```bash
curl -X PATCH http://localhost:8083/api/v1/orders/<ORDER_ID>/status \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "confirmed"}'
```

---

### 6. ขอยกเลิกออเดอร์ (`POST /api/v1/orders/:id/cancel`)
(ทำได้เฉพาะสถานะ pending หรือ confirmed)
```bash
curl -X POST http://localhost:8083/api/v1/orders/<ORDER_ID>/cancel \
  -H "Authorization: Bearer $TOKEN"
```
