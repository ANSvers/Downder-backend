downder-backend/ (Root)
├── cmd/
│   └── api/
│       └── main.go        # Entry point: จุดเริ่มต้นเชื่อมต่อ DB/Config และสั่งรัน Server
├── config/
│   └── config.go      # โหลด Environment Variables (.env) และ App Configuration
├── internal/
│   ├── delivery/
│   │   └── http/          # [Layer 1: Entry Point] รับ Request จากภายนอก (CORS, REST API)
│   │       ├── router.go  # จุดลงทะเบียนเส้นทางเดินของ API ทั้งหมด
│   │       ├── health.go  # Handler ตรวจสอบสถานะเซิร์ฟเวอร์
│   │       └── video.go   # Handler รับคิวงาน Fetch/Download และส่ง Response กลับหน้าบ้าน
│   ├── domain/            # [Layer 2: Core Domain] นิยามโครงสร้างข้อมูลและ Interface
│   │   └── video.go       # Structs payload และ Interface ของ Service/Scraper
│   ├── service/           # [Layer 3: Business Logic] สมองหลักของระบบ (ห้ามยุ่งกับ HTTP)
│   │   ├── processor.go   # สั่งตัดวิดีโอ, แยก MP3, แปลงไฟล์ผ่าน FFmpeg Wrapper
│   │   └── video.go       # คุม Logic ว่าถ้าลิงก์นี้มา ต้องไปหา Scraper ตัวไหน แล้วส่งไปประมวลผลต่อ
│   └── platform/          # [Layer 4: Infrastructure/External App] ตัวเชื่อมต่อภายนอก
│       ├── scraper/       # บอทแกะรอยวิดีโอแยกตามแต่ละแพลตฟอร์ม
│       │   ├── factory.go # ตัวเลือกว่าจะใช้ Scraper ตัวไหนตาม URL
│       │   ├── youtube.go
│       │   ├── tiktok.go
│       │   ├── instagram.go
│       │   ├── facebook.go
│       │   └── twitter.go
│       └── storage/       # ตัวจัดการไฟล์วิดีโอชั่วคราวในดิสก์
│           └── local.go   # สั่งลบไฟล์อัตโนมัติ (TTL) หรือล้างข้อมูลขยะ
├── pkg/                   # [Shared Libraries] ฟังก์ชันส่วนกลางที่หยิบไปใช้ที่อื่นได้ (ไม่มี Business Logic)
│   ├── ffmpeg/            # Low-level Exec Command สำหรับเรียกใช้ FFmpeg
│   └── qrcode/            # ตัวเจนเนอเรต QR Code เป็นรูปภาพหรือ Base64
├── .gitignore
├── Dockerfile
├── go.mod
└── go.sum