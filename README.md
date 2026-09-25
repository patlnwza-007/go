# ระบบสั่งอาหารและจัดการร้านอาหารออนไลน์ (Go Microservices)
## ส่วนที่ 3: Order Service (`order-service`)

* **ผู้รับผิดชอบ:** นายณราธร เหมือนเหลา (คนที่ 3)
* **รหัสนักศึกษา:** 67114540174
* **Service Port:** `8083`
* **Database:** PostgreSQL (`order_db`)

---

## 🛠️ โครงสร้างไฟล์ในโปรเจกต์
โปรเจกต์นี้ออกแบบให้เข้าใจง่าย ไม่ซับซ้อน เหมาะสำหรับส่งงานและอธิบายอาจารย์:
```
go_lang_project/
├── main.go            # เส้นทาง API (Routes) และฟังก์ชันจัดการคำสั่งซื้อ (Handlers)
├── models.go          # โครงสร้างตาราง orders, order_items และข้อมูลผู้พัฒนา
├── database.go        # ฟังก์ชันเชื่อมต่อ PostgreSQL (order_db) และสร้างตารางอัตโนมัติ
├── schema.sql         # คำสั่ง SQL DDL สำหรับสร้างตาราง orders และ order_items
├── docker-compose.yml # ไฟล์สำหรับรันฐานข้อมูล PostgreSQL บน Docker
├── .env               # ไฟล์กำหนดค่าพอร์ตและการเชื่อมต่อฐานข้อมูล
└── go.mod             # ไฟล์จัดการ Go Dependencies
```

---

## 🚀 วิธีการรันโปรเจกต์

### ขั้นตอนที่ 1: เปิดฐานข้อมูล PostgreSQL
เลือกวิธีใดวิธีหนึ่ง:

**วิธี A (แนะนำ - รันผ่าน Docker):**
```bash
docker compose up -d
```

**วิธี B (รันผ่าน Homebrew PostgreSQL ในเครื่อง Mac):**
```bash
brew services start postgresql@16
createdb order_db
```

---

### ขั้นตอนที่ 2: รัน Order Service
เปิด Terminal ในโฟลเดอร์โปรเจกต์ แล้วสั่ง:
```bash
go run .
```
ระบบจะเชื่อมต่อฐานข้อมูล `order_db`, สร้างตารางให้อัตโนมัติ (AutoMigrate) และเปิดให้บริการที่พอร์ต `http://localhost:8083`

---

## 📡 ตัวอย่างคำสั่งทดสอบ API (curl)

### 1. ดูข้อมูลผู้พัฒนา (คุณณราธร เหมือนเหลา)
```bash
curl http://localhost:8083/narathon
```
*หรือเปิดผ่านเบราว์เซอร์:* `http://localhost:8083/narathon`

---

### 2. ตรวจสอบสถานะ Service (Health Check)
```bash
curl http://localhost:8083/health
```

---

### 3. สั่งซื้ออาหาร (สร้าง Order พร้อม Snapshot รายการอาหาร)
```bash
curl -X POST http://localhost:8083/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
    "restaurant_id": "b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b22",
    "delivery_address": "อาคารเรียนรวม 2 มหาวิทยาลัย",
    "payment_method": "promptpay",
    "items": [
      {
        "menu_item_id": "c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c33",
        "item_name": "ข้าวกะเพราหมูกรอบ",
        "unit_price": 65.00,
        "qty": 2,
        "note": "เผ็ดน้อย ไข่ดาวสุก"
      },
      {
        "menu_item_id": "d3eebc99-9c0b-4ef8-bb6d-6bb9bd380d44",
        "item_name": "ชาไทยเย็น",
        "unit_price": 35.00,
        "qty": 1,
        "note": "หวานน้อย 50%"
      }
    ]
  }'
```

---

### 4. ดึงรายการออเดอร์ทั้งหมด
```bash
curl http://localhost:8083/orders
```
*สามารถกรองตามลูกค้าได้ เช่น:* `curl "http://localhost:8083/orders?customer_id=a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"`

---

### 5. ดูรายละเอียดออเดอร์ตาม ID
```bash
curl http://localhost:8083/orders/<ORDER_ID>
```

---

### 6. อัปเดตสถานะออเดอร์
สถานะที่อนุญาต: `pending`, `confirmed`, `cooking`, `ready`, `completed`, `cancelled`
```bash
curl -X PATCH http://localhost:8083/orders/<ORDER_ID>/status \
  -H "Content-Type: application/json" \
  -d '{
    "status": "cooking"
  }'
```

---

### 7. ขอยกเลิกออเดอร์
```bash
curl -X POST http://localhost:8083/orders/<ORDER_ID>/cancel
```
