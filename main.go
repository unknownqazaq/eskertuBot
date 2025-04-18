package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"log"
	"net/http"
	"os"
	"time"
)

// Tenant описывает данные арендатора
type Tenant struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Apartment   string `json:"apartment"`
	PaymentDate string `json:"paymentDate"` // формат "2006-01-02"
}

var db *sql.DB

//func initPostgres() error {
//	dsn := fmt.Sprintf(
//		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
//		os.Getenv("POSTGRES_HOST"),
//		os.Getenv("POSTGRES_PORT"),
//		os.Getenv("POSTGRES_USER"),
//		os.Getenv("POSTGRES_PASSWORD"),
//		os.Getenv("POSTGRES_DB"),
//	)
//	var err error
//	db, err = sql.Open("postgres", dsn)
//	if err != nil {
//		return err
//	}
//	return db.Ping()
//}

func initPostgres() error {
	dsn := os.Getenv("DATABASE_URL")
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	return db.Ping()
}

func migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS tenants (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		apartment TEXT NOT NULL,
		chat_id BIGINT UNIQUE,
		payment_date DATE NOT NULL
	);`
	_, err := db.Exec(query)
	return err
}

func sendTelegramMessage(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// Читаем все chat_id из базы данных
	rows, err := db.Query("SELECT chat_id FROM tenants WHERE chat_id IS NOT NULL")
	if err != nil {
		return fmt.Errorf("не удалось получить chat_id из базы данных: %v", err)
	}
	defer rows.Close()

	// Проходим по всем chat_id и отправляем сообщение каждому пользователю
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			log.Printf("Ошибка чтения chat_id: %v", err)
			continue
		}

		body, _ := json.Marshal(map[string]string{
			"chat_id": fmt.Sprintf("%d", chatID),
			"text":    message,
		})
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Ошибка отправки для chat_id %d: %v", chatID, err)
			continue
		}
		resp.Body.Close()
	}

	return nil
}

func checkPaymentDates() {
	now := time.Now().Truncate(24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	threeDaysLater := now.Add(72 * time.Hour)

	fmt.Println("Проверка дат оплаты на", now.Format("2006-01-02"))

	rows, err := db.Query(`SELECT name, apartment, payment_date FROM tenants`)
	if err != nil {
		log.Printf("Ошибка запроса к базе: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t Tenant
		var pd time.Time

		if err := rows.Scan(&t.Name, &t.Apartment, &pd); err != nil {
			log.Println("Ошибка чтения строки:", err)
			continue
		}
		t.PaymentDate = pd.Format("2006-01-02")

		if pd.Equal(threeDaysLater) {
			message := fmt.Sprintf("Привет, %s! Через 3 дня (%s) нужно оплатить аренду квартиры %s.",
				t.Name, pd.Format("02 January 2006"), t.Apartment)
			if err := sendTelegramMessage(message); err != nil {
				log.Printf("Ошибка отправки (за 3 дня) для %s: %v\n", t.Name, err)
			}
		} else if pd.Equal(tomorrow) {
			message := fmt.Sprintf("Привет, %s! Завтра (%s) нужно оплатить аренду квартиры %s.",
				t.Name, pd.Format("02 January 2006"), t.Apartment)
			if err := sendTelegramMessage(message); err != nil {
				log.Printf("Ошибка отправки (за 1 день) для %s: %v\n", t.Name, err)
			}
		}
	}
}

func startBot() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = true
	log.Printf("Авторизация как %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for update := range updates {
			if update.Message != nil {
				if update.Message.Text == "/start" {
					chatID := update.Message.Chat.ID

					// Сохраняем chatID в базу данных
					_, err := db.Exec("INSERT INTO tenants (chat_id) VALUES ($1) ON CONFLICT (chat_id) DO NOTHING", chatID)
					if err != nil {
						log.Println("Ошибка сохранения chat_id в базу данных:", err)
					}

					msg := tgbotapi.NewMessage(chatID, "Бот активирован. Вы будете получать уведомления о платежах.")
					bot.Send(msg)
				}
			}
		}
	}()
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Ошибка загрузки .env файла:", err)
	}

	if err := initPostgres(); err != nil {
		log.Fatalf("Ошибка подключения к PostgreSQL: %v", err)
	}
	if err := migrate(); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	c := cron.New()
	_, err := c.AddFunc("0 9 * * *", func() {
		fmt.Println("Запуск проверки арендаторов в 09:00")
		checkPaymentDates()
	})
	if err != nil {
		fmt.Println("Ошибка cron:", err)
	}
	c.Start()

	go startBot()

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://eskertu-bot.vercel.app"},
		AllowMethods:     []string{"POST", "GET", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))

	router.POST("/api/tenants", func(c *gin.Context) {
		var t Tenant
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
			return
		}

		paymentDate, err := time.Parse("2006-01-02", t.PaymentDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат даты"})
			return
		}

		query := `INSERT INTO tenants (name, apartment, payment_date) VALUES ($1, $2, $3)`
		_, err = db.Exec(query, t.Name, t.Apartment, paymentDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения в БД"})
			return
		}

		message := fmt.Sprintf("📢 Новый квартирант:\n👤 Имя: %s\n🏠 Квартира: %s\n💰 Дата оплаты: %s",
			t.Name, t.Apartment, t.PaymentDate)
		if err := sendTelegramMessage(message); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка Telegram", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Арендатор добавлен и уведомление отправлено",
			"tenant":  t,
		})
	})

	router.GET("/api/tenants", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, name, apartment, payment_date FROM tenants ORDER BY id`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения из БД"})
			return
		}
		defer rows.Close()

		var tenants []Tenant
		for rows.Next() {
			var t Tenant
			var pd time.Time
			if err := rows.Scan(&t.ID, &t.Name, &t.Apartment, &pd); err != nil {
				continue
			}
			t.PaymentDate = pd.Format("2006-01-02")
			tenants = append(tenants, t)
		}

		c.JSON(http.StatusOK, tenants)
	})

	router.PUT("/api/tenants/:id", func(c *gin.Context) {
		id := c.Param("id")
		var t Tenant
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
			return
		}
		pd, err := time.Parse("2006-01-02", t.PaymentDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат даты"})
			return
		}
		query := `UPDATE tenants SET name=$1, apartment=$2, payment_date=$3 WHERE id=$4`
		_, err = db.Exec(query, t.Name, t.Apartment, pd, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Арендатор обновлён"})
	})

	router.DELETE("/api/tenants/:id", func(c *gin.Context) {
		id := c.Param("id")
		_, err := db.Exec(`DELETE FROM tenants WHERE id=$1`, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Арендатор удалён"})
	})

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	router.Run(":8080")
}
