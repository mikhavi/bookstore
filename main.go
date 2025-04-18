package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB
var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.ParseFiles("template.html", "combined_view.html", "admin_main.html", "admin_view.html", "admin_edit.html", "admin_reports.html", "report_view.html", "user_reports.html", "queries.html", "query_result.html", "admin_procedures.html", "procedure_result.html") // Загрузка шаблонов
	if err != nil {
		panic("Ошибка загрузки шаблонов: " + err.Error())
	}
}

func containsSQLKeywords(input string) bool {
	sqlKeywords := []string{"SELECT", "UPDATE", "INSERT", "DELETE", "DROP", "ALTER", "CREATE", "EXEC", "UNION", "TRUNCATE"}
	for _, keyword := range sqlKeywords {
		if strings.Contains(strings.ToUpper(input), keyword) {
			return true
		}
	}
	return false
}

// Функция для получения роли пользователя
func getUserRole(login string) (string, error) {
	var role string
	// Попытка получить роль через таблицу users
	err := db.Table("users").Select("user_roles").Where("login = ?", login).Scan(&role).Error
	if err == nil {
		return role, nil // Если запрос успешен, возвращаем роль
	}
	// Если запрос к users не удался, пробуем получить данные через представление v_user3_view
	err = db.Table("v_user3_view").Select("user_roles").Where("login = ?", login).Scan(&role).Error
	if err != nil {
		return "", fmt.Errorf("Ошибка проверки роли пользователя: %w", err)
	}

	return role, nil // Возвращаем роль, если запрос через представление успешен
}

// Получение столбцов таблицы
func getTableColumns(tableName string) ([]string, error) {
	var columns []string
	err := db.Raw(`
        SELECT COLUMN_NAME
        FROM INFORMATION_SCHEMA.COLUMNS
        WHERE TABLE_NAME = ?
    `, tableName).Pluck("COLUMN_NAME", &columns).Error
	if err != nil {
		return nil, fmt.Errorf("Ошибка получения столбцов таблицы: %w", err)
	}
	return columns, nil
}

// ConvertToDecimal takes an interface value and tries to convert it to a proper decimal representation
func ConvertToDecimal(value interface{}) (string, error) {
	// Handle case where value is []byte
	if b, ok := value.([]byte); ok {
		decimalValue, err := decimal.NewFromString(string(b))
		if err != nil {
			return "", fmt.Errorf("error converting []byte to decimal: %v", err)
		}
		return decimalValue.String(), nil // Return properly formatted decimal string
	}

	// Handle case where value is a string
	if s, ok := value.(string); ok {
		decimalValue, err := decimal.NewFromString(s)
		if err != nil {
			return "", fmt.Errorf("error converting string to decimal: %v", err)
		}
		return decimalValue.String(), nil // Return properly formatted decimal string
	}

	// If the value is already in a numeric format, attempt conversion
	if f, ok := value.(float64); ok {
		return fmt.Sprintf("%.2f", f), nil // Format the float to two decimal places
	}

	// Return an error if the type is unsupported
	return "", fmt.Errorf("unsupported type: %T", value)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "template.html", nil)
	})

	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка парсинга формы"})
			return
		}

		user := r.FormValue("user")
		password := r.FormValue("password")

		if containsSQLKeywords(user) || containsSQLKeywords(password) {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка: Вводить SQL-запросы в поля пользователь и пароль запрещено!"})
			return
		}

		server := "localhost"
		port := "1433"
		database := "bookstore"

		connStr := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", user, password, server, port, database)
		dbTemp, err := gorm.Open(sqlserver.Open(connStr), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка подключения: " + err.Error()})
			return
		}

		db = dbTemp

		sqlDB, _ := db.DB()
		err = sqlDB.Ping()
		if err != nil {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Не удалось подключиться: " + err.Error()})
			return
		}

		// Получение роли пользователя
		role, err := getUserRole(user)
		if err != nil {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка получения роли пользователя: " + err.Error()})
			return
		}

		if role == "user" {
			tables := []string{"Classifier", "Publishers", "Books", "Authors", "AuthorNames", "Editions", "Warehouse"}
			tmpl.ExecuteTemplate(w, "combined_view.html", map[string]interface{}{"Tables": tables, "Message": "✅ Успешное подключение как пользователь!"})
		} else if role == "admin" {
			tables := []string{"Classifier", "Publishers", "Books", "Authors", "AuthorNames", "Editions", "Warehouse", "Orders", "Sales", "Employees"}
			tmpl.ExecuteTemplate(w, "admin_main.html", map[string]interface{}{"Tables": tables, "Message": "✅ Успешное подключение как администратор!"})
		} else {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Роль пользователя неизвестна или доступ запрещён!"})
		}
	})

	// Обработчик для admin_view
	http.HandleFunc("/admin_view", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "admin_main.html", map[string]string{"Message": "Ошибка парсинга формы"})
			return
		}

		tableName := r.FormValue("tableName")
		fmt.Println("Просмотр таблицы:", tableName)

		var records []map[string]interface{}
		err := db.Table(tableName).Find(&records).Error
		if err != nil {
			tmpl.ExecuteTemplate(w, "admin_main.html", map[string]string{"Message": "Ошибка выполнения запроса: " + err.Error()})
			return
		}

		data := map[string]interface{}{
			"Message":   "✅ Данные успешно получены!",
			"TableName": tableName,
			"Rows":      records,
		}
		tmpl.ExecuteTemplate(w, "admin_view.html", data)
	})
	// выбор отчетов для админа
	http.HandleFunc("/admin_reports", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Запрос к /admin_reports")
		// Формируем контекст для выбора отчета
		data := map[string]interface{}{
			"Message": "Выберите отчет для просмотра.",
			"Reports": []string{
				"Отчет по продажам сотрудника",
				"Список книг по автору",
				"Книги по разделу классификатора на складе",
			},
		}
		tmpl.ExecuteTemplate(w, "admin_reports.html", data)
	})
	http.HandleFunc("/view_report", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "admin_reports.html", map[string]string{"Message": "Ошибка при выборе отчета."})
			return
		}

		reportType := r.FormValue("reportType")
		filterValue := r.FormValue("filterValue") // Получаем значение, введенное пользователем
		var reportData []map[string]interface{}
		var query string
		if containsSQLKeywords(filterValue) {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка: Вводить SQL-запросы в поля пользователь и пароль запрещено!"})
			return
		}
		// Формируем запрос в зависимости от типа отчета
		switch reportType {
		case "v_SalesByEmployeeAndDate":
			query = fmt.Sprintf("SELECT * FROM v_SalesByEmployeeAndDate WHERE FullName LIKE '%%%s%%'", filterValue)
		case "v_BooksByAuthor":
			query = fmt.Sprintf("SELECT * FROM v_BooksByAuthor WHERE FullName LIKE '%%%s%%'", filterValue) // LIKE для поиска
		case "v_ClassifierBooksInWarehouse":
			query = fmt.Sprintf("SELECT * FROM v_BooksInStockByClassifier WHERE Name LIKE '%%%s%%'", filterValue)
		default:
			tmpl.ExecuteTemplate(w, "admin_reports.html", map[string]string{"Message": "Неизвестный тип отчета."})
			return
		}

		// Выполняем запрос
		err := db.Raw(query).Scan(&reportData).Error
		if err != nil {
			tmpl.ExecuteTemplate(w, "admin_reports.html", map[string]string{"Message": "Ошибка получения данных из представления: " + err.Error()})
			return
		}

		// Передаем данные в шаблон для отображения
		data := map[string]interface{}{
			"Message":    "✅ Данные успешно получены.",
			"ReportType": reportType,
			"ReportData": reportData,
		}
		tmpl.ExecuteTemplate(w, "report_view.html", data)
	})
	// просмотр отчетов для пользователя
	http.HandleFunc("/user_reports", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Обработчик /user_reports вызван")

		data := map[string]interface{}{
			"Message": "Выберите отчет для просмотра.",
		}

		err := tmpl.ExecuteTemplate(w, "user_reports.html", data)
		if err != nil {
			fmt.Println("Ошибка выполнения шаблона:", err)
			http.Error(w, "Ошибка выполнения шаблона: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})
	http.HandleFunc("/view_user_report", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "user_reports.html", map[string]string{"Message": "Ошибка при выборе отчета."})
			return
		}

		reportType := r.FormValue("reportType")
		filterValue := r.FormValue("filterValue")
		var reportData []map[string]interface{}
		var query string

		// Формируем запрос в зависимости от типа отчета
		switch reportType {
		case "v_BooksByAuthor":
			query = fmt.Sprintf("SELECT * FROM v_BooksByAuthor WHERE FullName LIKE '%%%s%%'", filterValue)
		case "v_ClassifierBooksInWarehouse":
			query = fmt.Sprintf("SELECT * FROM v_BooksInStockByClassifier WHERE Name LIKE '%%%s%%'", filterValue)
		default:
			tmpl.ExecuteTemplate(w, "user_reports.html", map[string]string{"Message": "Неизвестный тип отчета."})
			return
		}

		// Выполняем запрос
		err := db.Raw(query).Scan(&reportData).Error
		if err != nil {
			tmpl.ExecuteTemplate(w, "user_reports.html", map[string]string{"Message": "Ошибка получения данных из представления: " + err.Error()})
			return
		}

		// Передаем данные в шаблон для отображения
		data := map[string]interface{}{
			"Message":    "✅ Данные успешно получены.",
			"ReportType": reportType,
			"ReportData": reportData,
		}
		tmpl.ExecuteTemplate(w, "report_view.html", data)
	})

	//запросы
	http.HandleFunc("/queries", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Message": "Выберите запрос и при необходимости введите значение.",
		}
		err := tmpl.ExecuteTemplate(w, "queries.html", data)
		if err != nil {
			http.Error(w, "Ошибка загрузки страницы запросов: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/execute_query", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "queries.html", map[string]string{"Message": "Ошибка при отправке данных формы."})
			return
		}

		queryType := r.FormValue("queryType")
		inputValue := r.FormValue("inputValue")
		var query string
		var queryResult []map[string]interface{}

		if containsSQLKeywords(inputValue) {
			tmpl.ExecuteTemplate(w, "template.html", map[string]string{"Message": "Ошибка: Вводить SQL-запросы в поля пользователь и пароль запрещено!"})
			return
		}
		// Define query logic based on the selected query type
		switch queryType {
		case "totalBookCost":
			// Updated query to calculate total value of books in the warehouse
			query = `
            SELECT 
                s.BookCode,
                b.Name AS BookName,
                SUM(s.Price * s.NumberOfCopies) AS TotalValue
            FROM Warehouse s
            JOIN Books b ON s.BookCode = b.BookCode
            GROUP BY s.BookCode, b.Name;
        `
		case "employeeSalesCount":
			query = `
            SELECT e.FullName AS EmployeeName, SUM(s.Quantity) AS TotalSold
            FROM Sales s
            JOIN Employees e ON s.EmployeeID = e.EmployeeID
            GROUP BY e.FullName
        `
		case "customersByLetter":
			query = fmt.Sprintf(`
            SELECT CustomerInfo
            FROM Orders
            WHERE CustomerInfo LIKE '%s%%'
        `, inputValue) // Filtering by first letter
		case "publishersByDate":
			query = fmt.Sprintf(`
            SELECT DISTINCT p.Name AS PublisherName, p.PublisherCode, s.SaleDate
            FROM Sales s
            JOIN Books b ON s.BookID = b.BookCode
            JOIN Publishers p ON s.PublisherID = p.PublisherCode
            WHERE s.SaleDate = '%s'
        `, inputValue) // Filtering by date
		case "booksSoldOnDate":
			query = fmt.Sprintf(`
            SELECT b.Name AS BookTitle, s.Quantity
            FROM Sales s
            JOIN Books b ON s.BookID = b.BookCode
            WHERE s.SaleDate = '%s' AND s.IsOrder = 1
        `, inputValue) // Filtering by date and preorder
		default:
			tmpl.ExecuteTemplate(w, "queries.html", map[string]string{"Message": "Неизвестный тип запроса."})
			return
		}

		// Execute the query
		err := db.Raw(query).Scan(&queryResult).Error
		if err != nil {
			tmpl.ExecuteTemplate(w, "queries.html", map[string]string{"Message": "Ошибка выполнения запроса: " + err.Error()})
			return
		}

		// Render the query result
		data := map[string]interface{}{
			"Message":     "✅ Запрос выполнен успешно.",
			"QueryResult": queryResult,
		}
		tmpl.ExecuteTemplate(w, "query_result.html", data)
	})

	// изменение строк
	http.HandleFunc("/admin_edit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tableName := r.URL.Query().Get("tableName")

			var records []map[string]interface{}
			err := db.Table(tableName).Find(&records).Error
			if err != nil {
				tmpl.ExecuteTemplate(w, "admin_main.html", map[string]string{"Message": "Ошибка выполнения запроса: " + err.Error()})
				return
			}

			data := map[string]interface{}{
				"Message":   "✅ Данные успешно получены для редактирования!",
				"TableName": tableName,
				"Rows":      records,
			}
			tmpl.ExecuteTemplate(w, "admin_edit.html", data)
		} else if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "Ошибка парсинга формы"})
				return
			}

			tableName := r.FormValue("tableName")
			keyColumn := r.FormValue("keyColumn")
			keyValue := r.FormValue("keyValue")
			columnName := r.FormValue("columnName")
			newValue := r.FormValue("newValue")

			err := db.Table(tableName).Where(fmt.Sprintf("%s = ?", keyColumn), keyValue).Update(columnName, newValue).Error
			if err != nil {
				tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "Ошибка обновления таблицы: " + err.Error()})
				return
			}

			tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "✅ Изменения сохранены в базе данных!"})
		}
	})

	// Обработчик для удаления строки
	http.HandleFunc("/delete_row", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "Ошибка парсинга формы"})
				return
			}

			tableName := r.FormValue("tableName")
			keyColumn := r.FormValue("keyColumn")
			keyValue := r.FormValue("keyValue")

			err := db.Table(tableName).Where(fmt.Sprintf("%s = ?", keyColumn), keyValue).Delete(nil).Error
			if err != nil {
				tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "Ошибка удаления строки: " + err.Error()})
				return
			}

			tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]string{"Message": "✅ Строка успешно удалена!"})
		}
	})
	//добавление строки
	http.HandleFunc("/add_row", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tableName := r.URL.Query().Get("tableName")

			columns, err := getTableColumns(tableName)
			if err != nil {
				tmpl.ExecuteTemplate(w, "admin_main.html", map[string]string{"Message": "Ошибка получения столбцов таблицы: " + err.Error()})
				return
			}

			data := map[string]interface{}{
				"Message":   "✅ Столбцы успешно получены. Заполните поля для добавления новой строки.",
				"TableName": tableName,
				"Columns":   columns,
			}
			tmpl.ExecuteTemplate(w, "admin_edit.html", data)
		}
	})
	//процедуры
	http.HandleFunc("/admin_procedures", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Message": "Выберите хранимую процедуру для выполнения.",
		}
		err := tmpl.ExecuteTemplate(w, "admin_procedures.html", data)
		if err != nil {
			http.Error(w, "Ошибка загрузки страницы процедур: "+err.Error(), http.StatusInternalServerError)
		}
	})
	http.HandleFunc("/execute_procedure", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			tmpl.ExecuteTemplate(w, "admin_procedures.html", map[string]string{"Message": "Ошибка при обработке формы."})
			return
		}

		procedureName := r.FormValue("procedureName")
		inputValue := r.FormValue("inputValue") // Optional input for some procedures
		var query string
		var procedureResult []map[string]interface{}

		// Define logic to execute the procedure
		switch procedureName {
		case "GetExpensiveStockBooks":
			query = "EXEC GetExpensiveStockBooks"
		case "GetOrderDetails":
			if inputValue == "" {
				tmpl.ExecuteTemplate(w, "admin_procedures.html", map[string]string{"Message": "Для данной процедуры требуется значение!"})
				return
			}
			query = fmt.Sprintf("EXEC GetOrderDetails @OrderID = %s", inputValue)
		case "InsertPublishers":
			query = "EXEC InsertPublishers"
		case "CalculateAdditionalPayment":
			if inputValue == "" {
				tmpl.ExecuteTemplate(w, "admin_procedures.html", map[string]string{"Message": "Для данной процедуры требуется значение!"})
				return
			}
			query = fmt.Sprintf("EXEC CalculateAdditionalPayment @BookCode = %s", inputValue)
		default:
			tmpl.ExecuteTemplate(w, "admin_procedures.html", map[string]string{"Message": "Неизвестная процедура."})
			return
		}

		// Execute the query
		rows, err := db.Raw(query).Rows()
		if err != nil {
			tmpl.ExecuteTemplate(w, "admin_procedures.html", map[string]string{"Message": "Ошибка выполнения процедуры: " + err.Error()})
			return
		}
		defer rows.Close()

		// Parse the result and handle decimal values
		columns, _ := rows.Columns()
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			rows.Scan(valuePtrs...)

			result := make(map[string]interface{})
			for i, col := range columns {
				// Handle the AdditionalPayment column separately
				if col == "AdditionalPayment" {
					formattedValue, err := ConvertToDecimal(values[i]) // Format decimal value
					if err == nil {
						result[col] = formattedValue // Properly formatted decimal value
					} else {
						result[col] = values[i] // Fallback for unsupported types
					}
				} else {
					result[col] = values[i] // Default processing for other columns
				}
			}
			procedureResult = append(procedureResult, result)
		}

		// Render the procedure result
		data := map[string]interface{}{
			"Message":         "✅ Процедура выполнена успешно.",
			"ProcedureResult": procedureResult,
		}
		tmpl.ExecuteTemplate(w, "procedure_result.html", data)
	})

	fmt.Println("Сервер запущен на http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
