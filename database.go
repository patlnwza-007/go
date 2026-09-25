package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// อ่านไฟล์ .env อัตโนมัติ (ถ้ามี)
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func InitDB() {
	loadEnvFile()

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5434")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "order_db")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Bangkok",
		host, user, password, dbname, port,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Printf("⚠️ ไม่สามารถเชื่อมต่อฐานข้อมูล %s ได้ทันที: %v", dbname, err)
		log.Println("👉 แนะนำ: รัน 'docker compose up -d' เพื่อเปิดฐานข้อมูล PostgreSQL")
		return
	}

	log.Printf("✅ เชื่อมต่อฐานข้อมูล PostgreSQL (%s:%s/%s) สำเร็จ!", host, port, dbname)

	// สร้างตาราง orders และ order_items อัตโนมัติ (AutoMigrate)
	err = DB.AutoMigrate(&Order{}, &OrderItem{})
	if err != nil {
		log.Fatalf("❌ สร้างตารางฐานข้อมูลไม่สำเร็จ: %v", err)
	}

	log.Println("✅ ตรวจสอบและสร้างตาราง orders, order_items เรียบร้อยแล้ว")
}
