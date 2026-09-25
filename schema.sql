-- ==============================================================================
-- ระบบสั่งอาหารและจัดการร้านอาหารออนไลน์ ด้วยสถาปัตยกรรม Go Microservices
-- ส่วนที่ 3: Order Service (order-service) | Port: 8083 | Database: order_db
-- ผู้รับผิดชอบ: คนที่ 3 — นายณราธร เหมือนเหลา (รหัสนักศึกษา: 67114540174)
-- ==============================================================================

-- 1. สร้างฐานข้อมูล order_db (กรณีสร้างใหม่)
-- CREATE DATABASE order_db;

-- 2. ตาราง orders
-- หน้าที่: เก็บรายการสั่งอาหาร (คำสั่งซื้อ) แต่ละครั้ง พร้อมสถานะตลอดวงจรชีวิตของออเดอร์
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,                       -- อ้างอิง user_db.users.id (ไม่มี FK จริง ข้าม database)
    restaurant_id UUID NOT NULL,                     -- อ้างอิง restaurant_db.restaurants.id (ไม่มี FK จริง)
    delivery_address TEXT NOT NULL,                  -- ที่อยู่จัดส่ง
    payment_method VARCHAR(20) NOT NULL CHECK (      -- ช่องทางชำระเงิน
        payment_method IN ('cash', 'promptpay', 'credit_card')
    ),
    total_price DECIMAL(10, 2) NOT NULL,             -- ยอดรวม (คำนวณจากราคาที่ดึงมาตอนสั่ง)
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK ( -- สถานะออเดอร์
        status IN ('pending', 'confirmed', 'cooking', 'ready', 'completed', 'cancelled')
    ),
    created_at TIMESTAMP NOT NULL DEFAULT now(),     -- วันเวลาที่สั่ง
    updated_at TIMESTAMP NOT NULL DEFAULT now()      -- วันเวลาที่แก้ไขล่าสุด
);

-- 3. ตาราง order_items
-- หน้าที่: เก็บรายการอาหารย่อยในแต่ละออเดอร์ พร้อม "สแนปช็อต" ชื่อและราคาเมนู ณ เวลาที่สั่ง เพื่อไม่ให้ประวัติเปลี่ยนตามภายหลัง
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE, -- ออเดอร์ที่รายการนี้สังกัด (FK ภายใน DB เดียวกัน)
    menu_item_id UUID NOT NULL,                     -- อ้างอิง restaurant_db.menu_items.id (ไม่มี FK จริง)
    item_name VARCHAR(255) NOT NULL,                -- ชื่อเมนู ณ เวลาที่สั่ง (Snapshot)
    unit_price DECIMAL(10, 2) NOT NULL,             -- ราคาต่อหน่วย ณ เวลาที่สั่ง (Snapshot)
    qty INTEGER NOT NULL CHECK (qty >= 1 AND qty <= 20), -- จำนวนที่สั่ง (1-20)
    note VARCHAR(255),                              -- หมายเหตุต่อรายการ เช่น ไม่ใส่ผัก
    subtotal DECIMAL(10, 2) NOT NULL,               -- unit_price * qty
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- สร้าง Index เพื่อเพิ่มประสิทธิภาพในการ Query
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_restaurant_id ON orders(restaurant_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
