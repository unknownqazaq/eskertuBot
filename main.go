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
	"io/ioutil"
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

func initPostgres() error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_DB"),
	)

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
		payment_date DATE NOT NULL
	);`
	_, err := db.Exec(query)
	return err
}

// sendTelegramMessage отправляет Telegram-сообщение через Bot API
func sendTelegramMessage(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	body, _ := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    message,
	})

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("telegram error: %s", resBody)
	}

	return nil
}

// checkPaymentDates проходит по всем арендаторам и отправляет уведомление
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

// startBot запускает Telegram-бота (временно без команд)
func startBot() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = true
	log.Printf("Авторизация как %s", bot.Self.UserName)
}

func main() {
	// .env
	if err := godotenv.Load(); err != nil {
		log.Println("Ошибка загрузки .env файла:", err)
	}

	if err := initPostgres(); err != nil {
		log.Fatalf("Ошибка подключения к PostgreSQL: %v", err)
	}
	if err := migrate(); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	// Cron
	c := cron.New()
	_, err := c.AddFunc("0 9 * * *", func() {
		fmt.Println("Запуск проверки арендаторов в 09:00")
		checkPaymentDates()
	})
	if err != nil {
		fmt.Println("Ошибка cron:", err)
	}
	c.Start()

	// Telegram bot
	go startBot()

	// Gin
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5185", "https://eskertu-bot.vercel.app"},
		AllowMethods:     []string{"POST", "GET", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))

	// POST /api/tenants — добавление арендатора
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

	// GET /ping
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Запуск сервера
	router.Run(":8080")
}
