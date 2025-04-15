package main

import (
    "fmt"
    "html/template"
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

var tmpl = template.Must(template.New("form").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Подключение к MSSQL</title>
</head>
<body>
    <h2>Форма подключения к MSSQL</h2>
    <form method="POST" action="/connect">
        <label>Сервер (например: localhost):</label><br>
        <input type="text" name="server" required><br><br>

        <label>Порт (например: 1433):</label><br>
        <input type="text" name="port" required><br><br>

        <label>Пользователь:</label><br>
        <input type="text" name="user" required><br><br>

        <label>Пароль:</label><br>
        <input type="password" name="password" required><br><br>

        <label>База данных:</label><br>
        <input type="text" name="database" required><br><br>

        <label>Имя таблицы:</label><br>
        <input type="text" name="table" required><br><br>

        <button type="submit">Подключиться и вывести данные</button>
    </form>

    {{if .Message}}
        <p><strong>{{.Message}}</strong></p>
    {{end}}

    {{if .Rows}}
        <h3>Содержимое таблицы:</h3>
        <table border="1" cellpadding="5" cellspacing="0">
            <thead>
                <tr>
                    {{range .Columns}}
                        <th>{{.}}</th>
                    {{end}}
                </tr>
            </thead>
            <tbody>
                {{range .Rows}}
                    <tr>
                        {{range .}}
                            <td>{{.}}</td>
                        {{end}}
                    </tr>
                {{end}}
            </tbody>
        </table>
    {{end}}
</body>
</html>
`))

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        tmpl.Execute(w, nil)
    })

    http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
        if err := r.ParseForm(); err != nil {
            tmpl.Execute(w, map[string]string{"Message": "Ошибка парсинга формы"})
            return
        }

        server := r.FormValue("server")
        port := r.FormValue("port")
        user := r.FormValue("user")
        password := r.FormValue("password")
        database := r.FormValue("database")
        table := r.FormValue("table")

        connStr := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", user, password, server, port, database)
        dbTemp, err := gorm.Open(sqlserver.Open(connStr), &gorm.Config{})
        if err != nil {
            tmpl.Execute(w, map[string]string{"Message": "Ошибка подключения: " + err.Error()})
            return
        }

        db = dbTemp

        // Проверка соединения
        sqlDB, _ := db.DB()
        err = sqlDB.Ping()
        if err != nil {
            tmpl.Execute(w, map[string]string{"Message": "Не удалось подключиться: " + err.Error()})
            return
        }

        var records []Record
        err = db.Table(table).Find(&records).Error
        if err != nil {
            tmpl.Execute(w, map[string]string{"Message": "Ошибка выполнения запроса: " + err.Error()})
            return
        }

        columns := []string{"ID", "Name", "Other"}
        var resultRows [][]interface{}
        for _, record := range records {
            resultRows = append(resultRows, []interface{}{record.ID, record.Name, record.Other})
        }

        data := map[string]interface{}{
            "Message": "✅ Успешное подключение и получение данных!",
            "Columns": columns,
            "Rows":    resultRows,
        }
        tmpl.Execute(w, data)
    })

    fmt.Println("Сервер запущен на http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
