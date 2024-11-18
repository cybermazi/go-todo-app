package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Todo represents a single to-do item.
type Todo struct {
	ID        int       `json:"id"` 
	Task      string    `json:"task"`
	Completed bool      `json:"completed"`
	DueDate   time.Time `json:"due_date"`
	Category  string    `json:"category"`
}

var (
	db       *sql.DB
	tmpl     = template.Must(template.ParseFiles("templates/index.html"))
	tmplEdit = template.Must(template.ParseFiles("templates/edit.html"))
	mu       sync.Mutex
)

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./todos.db")
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task TEXT NOT NULL,
		completed BOOLEAN NOT NULL DEFAULT 0,
		due_date DATETIME,
		category TEXT
	);
	`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

func main() {
	initDB()
	defer db.Close()

	// Route handlers
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/add", addTodoHandler)
	http.HandleFunc("/delete", deleteTodoHandler)
	http.HandleFunc("/complete", completeTodoHandler)
	http.HandleFunc("/edit", editTodoHandler)

	// Static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start the server
	log.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// homeHandler serves the main page with the list of to-dos.
func homeHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, task, completed, due_date, category FROM todos ORDER BY due_date ASC")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var todo Todo
		var dueDate sql.NullTime
		err := rows.Scan(&todo.ID, &todo.Task, &todo.Completed, &dueDate, &todo.Category)
		if err != nil {
			http.Error(w, "Failed to parse tasks", http.StatusInternalServerError)
			return
		}
		if dueDate.Valid {
			todo.DueDate = dueDate.Time
		}
		todos = append(todos, todo)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Failed to retrieve tasks", http.StatusInternalServerError)
		return
	}

	// Pass tasks to the template
	if err := tmpl.Execute(w, todos); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// addTodoHandler handles the addition of new to-do items.
func addTodoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	task := r.FormValue("task")
	dueDateStr := r.FormValue("due_date")
	category := r.FormValue("category")

	if task == "" {
		http.Error(w, "Task cannot be empty", http.StatusBadRequest)
		return
	}

	var dueDate time.Time
	var err error
	if dueDateStr != "" {
		dueDate, err = time.Parse("2006-01-02", dueDateStr)
		if err != nil {
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
	}

	stmt, err := db.Prepare("INSERT INTO todos(task, completed, due_date, category) VALUES(?, ?, ?, ?)")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(task, false, dueDate, category)
	if err != nil {
		http.Error(w, "Failed to add task", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// deleteTodoHandler handles the deletion of to-do items.
func deleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("DELETE FROM todos WHERE id = ?")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(id)
	if err != nil {
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// completeTodoHandler handles toggling the completion status of to-do items.
func completeTodoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	// Retrieve current completion status
	var completed bool
	err = db.QueryRow("SELECT completed FROM todos WHERE id = ?", id).Scan(&completed)
	if err != nil {
		http.Error(w, "Failed to retrieve task", http.StatusInternalServerError)
		return
	}

	// Toggle completion status
	completed = !completed

	stmt, err := db.Prepare("UPDATE todos SET completed = ? WHERE id = ?")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(completed, id)
	if err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	// Respond with a redirect
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// editTodoHandler handles editing of to-do items.
func editTodoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		var todo Todo
		var dueDate sql.NullTime
		err = db.QueryRow("SELECT id, task, completed, due_date, category FROM todos WHERE id = ?", id).Scan(&todo.ID, &todo.Task, &todo.Completed, &dueDate, &todo.Category)
		if err != nil {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}

		if dueDate.Valid {
			todo.DueDate = dueDate.Time
		}

		tmplEdit.Execute(w, todo)
		return
	}

	if r.Method == http.MethodPost {
		idStr := r.FormValue("id")
		task := r.FormValue("task")
		dueDateStr := r.FormValue("due_date")
		category := r.FormValue("category")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		var dueDate time.Time
		if dueDateStr != "" {
			dueDate, err = time.Parse("2006-01-02", dueDateStr)
			if err != nil {
				http.Error(w, "Invalid date format", http.StatusBadRequest)
				return
			}
		}

		stmt, err := db.Prepare("UPDATE todos SET task = ?, due_date = ?, category = ? WHERE id = ?")
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(task, dueDate, category, id)
		if err != nil {
			http.Error(w, "Failed to update task", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}
