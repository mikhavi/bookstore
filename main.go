package main

import (
	"fmt"
	"net/http"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

type Record struct {
	// Объявляем структуру для записи в таблице
	// Поля должны соответствовать колонкам таблицы
	ID    int    `gorm:"column:id" json:"id"`
	Name  string `gorm:"column:name" json:"name"`
	Other string `gorm:"column:other" json:"other"`
}

var db *gorm.DB

func main() {
	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error": "Ошибка парсинга формы: %s"}`, err.Error())
			return
		}

		server := r.FormValue("server")
		port := r.FormValue("port")
		user := r.FormValue("user")
		password := r.FormValue("password")
		database := r.FormValue("database")
		table := r.FormValue("table")

		// Формируем строку подключения к MSSQL
		connStr := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", user, password, server, port, database)
		dbTemp, err := gorm.Open(sqlserver.Open(connStr), &gorm.Config{})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Ошибка подключения: %s"}`, err.Error())
			return
		}

		db = dbTemp

		// Проверка соединения
		sqlDB, _ := db.DB()
		err = sqlDB.Ping()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Не удалось подключиться: %s"}`, err.Error())
			return
		}

		var records []Record
		err = db.Table(table).Find(&records).Error
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "Ошибка выполнения запроса: %s"}`, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Возвращаем данные как JSON
		fmt.Fprintf(w, `{"message": "✅ Успешное подключение и получение данных!", "data": %v}`, records)
	})

	fmt.Println("Сервер запущен на http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
