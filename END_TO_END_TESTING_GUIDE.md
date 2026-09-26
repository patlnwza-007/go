# คู่มือการทดสอบระบบ Go Microservices แบบครบวงจร (End-to-End Testing Guide)
## จากเริ่มต้นไม่มีข้อมูล (0) จนถึงส่งอาหารถึงมือลูกค้าสำเร็จ (100)

เอกสารนี้จัดทำขึ้นสำหรับทีมพัฒนาทั้ง 4 คน เพื่อใช้เป็นคู่มือทดสอบและซักซ้อมการเชื่อมต่อ Service ร่วมกันจริง

---

## 👥 ข้อมูลสมาชิกและ Service ที่รับผิดชอบ

| ลำดับ | ผู้รับผิดชอบ | Service | Port | URL / ngrok | หน้าที่หลัก |
|:---:|---|---|:---:|---|---|
| **คนที่ 1** | นายชัยยศ ศรีสว่าง | Auth & User Service | `8081` | `https://pursuant-battered-untimed.ngrok-free.dev` | ออก JWT Token, จัดการ User |
| **คนที่ 2** | นางสาวกันยาดา ยุบล | Restaurant & Menu Service | `8082` | `https://reproduce-grandma-gangway.ngrok-free.dev` | จัดการร้านอาหาร, เมนูอาหาร |
| **คนที่ 3** | นายณราธร เหมือนเหลา | Order Service | `8083` | `https://rubdown-cardstock-nearby.ngrok-free.dev` | สั่งอาหาร, คิดเงิน, สถานะออเดอร์ |
| **คนที่ 4** | นายปรรณวิสิฐ บุญลพดิฐกรณ์ | Delivery Service | `8084` | `http://delivery-service:8084` | รับงานส่งอาหาร, ไรเดอร์จัดส่ง |

---

## 🗺️ แผนผังลำดับขั้นตอนการทดสอบ (Workflow)

```
[1. ขอ Token] ➔ [2. สร้างร้าน & เมนู] ➔ [3. สั่งซื้ออาหาร] ➔ [4. ปรุงอาหาร] ➔ [5. ไรเดอร์จัดส่ง] ➔ [6. ออเดอร์สำเร็จ]
  (คนที่ 1)           (คนที่ 2)             (คนที่ 3)           (คนที่ 3)          (คนที่ 4)           (เสร็จสิ้น)
```

---

## 🟢 ขั้นตอนที่ 1: เตรียม User และขอ Token (หน้าที่: คนที่ 1)
*เป้าหมาย: ขอ JWT Token ของ 4 บทบาทมาเก็บไว้ใช้*

ยิงคำสั่ง Login เพื่อรับ Token:
```bash
POST https://pursuant-battered-untimed.ngrok-free.dev/api/v1/auth/login
Content-Type: application/json
ngrok-skip-browser-warning: true

{
  "email": "admin@example.com",
  "password": "AdminPassword123!"
}
```
> 📌 **สิ่งที่ได้:** เก็บ `access_token` ไว้สำหรับใส่ใน Header `Authorization: Bearer <TOKEN>`

---

## 🟡 ขั้นตอนที่ 2: สร้างร้านค้าและเพิ่มเมนูอาหาร (หน้าที่: คนที่ 2)
*เป้าหมาย: ได้ `restaurant_id` และ `menu_item_id` ที่มีอยู่จริง*

### 2.1 เจ้าของร้านสร้างร้านอาหาร
* **Method:** `POST`
* **URL:** `https://reproduce-grandma-gangway.ngrok-free.dev/api/v1/restaurants`
* **Authorization:** `Bearer <TOKEN_RESTAURANT_OWNER>`
* **Body:**
```json
{
  "name": "ร้านกะเพราถาดยักษ์",
  "description": "กะเพราหมูกรอบสูตรโบราณ",
  "address": "หน้ามหาวิทยาลัย",
  "phone": "0812345678",
  "open_time": "09:00",
  "close_time": "20:00"
}
```
> 📌 **ผลลัพธ์:** ได้ **`restaurant_id`** (เช่น `eda47636-5d7a-4fb6-9a80-aadffeb4c614`)

---

### 2.2 Admin อนุมัติร้านค้า
* ให้ Admin อนุมัติสถานะร้านจาก `pending` ให้เป็น `approved` เพื่อเปิดขายสู่สาธารณะ

---

### 2.3 เจ้าของร้านเพิ่มเมนูอาหาร
* **Method:** `POST`
* **URL:** `https://reproduce-grandma-gangway.ngrok-free.dev/api/v1/restaurants/{restaurant_id}/menu-items`
* **Authorization:** `Bearer <TOKEN_RESTAURANT_OWNER>`
* **Body:**
```json
{
  "name": "ข้าวกะเพราหมูกรอบ",
  "description": "เผ็ดจัดจ้าน ไข่ดาวกรอบ",
  "price": 65.00,
  "category": "อาหารจานเดียว",
  "available": true
}
```
> 📌 **ผลลัพธ์:** ได้ **`menu_item_id`** (เช่น `13799010-7298-4c25-9cbe-07d928b0b9a0`)

---

## 🔵 ขั้นตอนที่ 3: สั่งซื้ออาหาร (หน้าที่: คนที่ 3 — ณราธร)
*เป้าหมาย: สร้างคำสั่งซื้อจริง โดย Order Service จะวิ่งไปดึงราคาจากคนที่ 2*

* **Method:** `POST`
* **URL:** `https://rubdown-cardstock-nearby.ngrok-free.dev/api/v1/orders`
* **Authorization:** `Bearer <TOKEN_CUSTOMER>`
* **Body:**
```json
{
  "restaurant_id": "<ใส่ restaurant_id จากขั้นตอน 2.1>",
  "delivery_address": "อาคารเรียนรวม 2 ชั้น 3 ห้อง 301 มหาวิทยาลัย",
  "payment_method": "promptpay",
  "items": [
    {
      "menu_item_id": "<ใส่ menu_item_id จากขั้นตอน 2.3>",
      "qty": 2,
      "note": "เผ็ดน้อย ไม่ใส่ผักชี"
    }
  ]
}
```
> 📌 **สิ่งที่ระบบ Order Service ทำ:**
> 1. ดึงราคา 65 บาทจากคนที่ 2 อัตโนมัติ
> 2. คิดยอดรวม: 65 × 2 = 130 บาท
> 3. บันทึก Snapshot ลงฐานข้อมูล `order_db`
> 4. ส่งคืน **`order_id`** และสถานะเป็น `"pending"`

---

## 🟣 ขั้นตอนที่ 4: ร้านค้ารับออเดอร์และทำอาหาร (หน้าที่: คนที่ 3 — ณราธร)
*เป้าหมาย: ขยับสถานะออเดอร์ตาม Lifecycle (ห้ามข้ามขั้น)*

เจ้าของร้านใช้ Token ของตนเอง ยิงปรับสถานะ:

### 4.1 รับออเดอร์ (`pending` ➔ `confirmed`)
* **PATCH:** `https://rubdown-cardstock-nearby.ngrok-free.dev/api/v1/orders/{order_id}/status`
* **Body:** `{"status": "confirmed"}`

### 4.2 เริ่มปรุงอาหาร (`confirmed` ➔ `cooking`)
* **PATCH:** `https://rubdown-cardstock-nearby.ngrok-free.dev/api/v1/orders/{order_id}/status`
* **Body:** `{"status": "cooking"}`

### 4.3 ปรุงเสร็จพร้อมส่ง (`cooking` ➔ `ready`)
* **PATCH:** `https://rubdown-cardstock-nearby.ngrok-free.dev/api/v1/orders/{order_id}/status`
* **Body:** `{"status": "ready"}`
> 📌 **ส่งต่อ:** แจ้งคนที่ 4 (Delivery Service) ว่าออเดอร์รหัสนี้ `ready` พร้อมให้ไรเดอร์มารับแล้ว

---

## 🟠 ขั้นตอนที่ 5: จัดส่งอาหาร (หน้าที่: คนที่ 4)
*เป้าหมาย: ไรเดอร์รับงานและนำอาหารไปส่ง*

1. **สร้างงานจัดส่ง:** ระบบ Delivery สร้างงานโดยอ้างอิง `order_id` (Delivery Service จะโทรมาตรวจสอบที่ Order Service ว่าออเดอร์ `ready` จริงหรือไม่)
2. **ไรเดอร์รับงาน:** ไรเดอร์กดรับงาน สถานะเปลี่ยนเป็น `assigned`
3. **รับอาหารที่ร้าน:** ไรเดอร์รับอาหารจากร้าน สถานะเปลี่ยนเป็น `picked_up`
4. **กำลังเดินทาง:** ไรเดอร์ขับรถไปส่ง สถานะเปลี่ยนเป็น `on_the_way`

---

## 🏁 ขั้นตอนที่ 6: ส่งอาหารสำเร็จและปิดออเดอร์
*เป้าหมาย: ออเดอร์เสร็จสิ้นสมบูรณ์*

1. **ไรเดอร์ส่งถึงมือลูกค้า:** อัปเดตงานจัดส่งใน Delivery Service เป็น **`delivered`**
2. **ปิดออเดอร์:** ออเดอร์ใน Order Service ปรับสถานะเป็น **`completed`**
   * **PATCH:** `https://rubdown-cardstock-nearby.ngrok-free.dev/api/v1/orders/{order_id}/status`
   * **Body:** `{"status": "completed"}`

---

## 📊 ตารางสรุป ID ที่ต้องส่งต่อกันในทีม

```
[คนที่ 1] ออก JWT Token ให้ทุกคน
   ↓
[คนที่ 2] ส่ง restaurant_id และ menu_item_id ให้ คนที่ 3
   ↓
[คนที่ 3] สั่งอาหารสำเร็จ ได้ order_id ส่งให้ คนที่ 4
   ↓
[คนที่ 4] จัดส่งอาหารจนสำเร็จ
```
