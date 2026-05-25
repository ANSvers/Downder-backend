downder-backend/ (Root)
├── cmd/
│   └── api/
│       └── main.go        # Entry point: จุดเริ่มต้นเชื่อมต่อ DB/Config และสั่งรัน Server
├── config/
│   └── config.go          # โหลด Environment Variables (.env) และ App Configuration
├── internal/
│   ├── delivery/
│   │   └── http/          # [Layer 1: Entry Point] รับ Request จากภายนอก (CORS, REST API)
│   │       ├── middleware/# จัดการ Rate Limit, CORS, Logger
│   │       ├── router.go  # จุดลงทะเบียนเส้นทางเดินของ API ทั้งหมด
│   │       ├── health.go  # Handler ตรวจสอบสถานะเซิร์ฟเวอร์
│   │       └── video_handler.go   # Handler รับคิวงาน Fetch/Download และส่ง Response กลับหน้าบ้าน
│   ├── domain/            # [Layer 2: Core Domain] นิยามโครงสร้างข้อมูลและ Interface
│   │   └── video.go       # Structs payload และ Interface ของ Service/Scraper
│   │   └── errors.go      # ศูนย์รวม Error ของระบบ
│   ├── service/           # [Layer 3: Business Logic] สมองหลักของระบบ (ห้ามยุ่งกับ HTTP)
│   │   ├── processor.go   # สั่งตัดวิดีโอ, แยก MP3, แปลงไฟล์ผ่าน FFmpeg Wrapper
│   │   └── video.go       # ไฟล์นี้จะรับลิงก์เข้ามา แล้วตรวจสอบว่าเป็นลิงก์ของเว็บไหน เพื่อส่งให้ Scraper ทำงานได้อย่างถูกต้อง
│   └── platform/          # [Layer 4: Infrastructure/External App] ตัวเชื่อมต่อภายนอก
│       ├── scraper/       # บอทแกะรอยวิดีโอแยกตามแต่ละแพลตฟอร์ม
│       │   ├── factory.go # ตัวเลือกว่าจะใช้ Scraper ตัวไหนตาม URL
│       │   ├── youtube.go
│       │   ├── tiktok.go
│       │   ├── instagram.go
│       │   ├── facebook.go
│       │   └── twitter.go
│       └── storage/       # ตัวจัดการไฟล์วิดีโอชั่วคราวในดิสก์
│           └── local.go   # จัดการเรื่องการเขียนไฟล์วิดีโอลงดิสก์อย่างปลอดภัย
│           └── cleaner.go # ลบไฟล์วิดีโอที่ผู้ใช้โหลดเสร็จแล้วทิ้งไป (กันเซิร์ฟเวอร์ดิสก์เต็ม)
├── pkg/                   # [Shared Libraries] ฟังก์ชันส่วนกลางที่หยิบไปใช้ที่อื่นได้ (ไม่มี Business Logic)
│   ├── ffmpeg/            # Low-level Exec Command สำหรับเรียกใช้ FFmpeg
│   └── qrcode/            # ตัวเจนเนอเรต QR Code เป็นรูปภาพหรือ Base64
├── .gitignore
├── Dockerfile
├── go.mod
└── go.sum
