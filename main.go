package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3" // use the same library with Egemen
)

type Task struct {
	ID       int    `json:"id"`
	Task     string `json:"task"`
	Deadline string `json:"deadline"`
}

type Server struct {
	db     *sql.DB
	logger *log.Logger
}

func NewServer(db *sql.DB, logger *log.Logger) *Server {
	return &Server{
		db:     db,
		logger: logger,
	}
}

func (s *Server) SaveTask(res http.ResponseWriter, req *http.Request) {
	var task Task
	err := json.NewDecoder(req.Body).Decode(&task)
	if err != nil {
		http.Error(res, "Failed to parse JSON body", http.StatusBadRequest)
		return
	}

	_, err = s.db.Exec("INSERT INTO reqs(task, deadline) VALUES (?, ?)", task.Task, task.Deadline)
	if err != nil {
		http.Error(res, "Failed to save task to database", http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusCreated)
	res.Write([]byte("Task saved successfully"))
}

func (s *Server) ListTasks(res http.ResponseWriter, req *http.Request) {
	rows, err := s.db.Query("SELECT id, task, deadline FROM reqs")
	if err != nil {
		http.Error(res, "Failed to fetch tasks from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.ID, &task.Task, &task.Deadline)
		if err != nil {
			http.Error(res, "Failed to scan tasks from database", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(tasks)
}

func (s *Server) InitializeDB() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS reqs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task TEXT NOT NULL,
		deadline TEXT NOT NULL
	);`)
	return err
}


func (s *Server) Run(ctx context.Context, addr string) error {

	err := s.InitializeDB()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/save_task", s.SaveTask)
	mux.HandleFunc("/task_list", s.ListTasks)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		s.logger.Printf("listening on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("error listening and serving: %s\n", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 17*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		s.logger.Fatalf("error shutting down http server: %s \n", err)
		return err
	}

	return nil
}

func main() {
	db, err := sql.Open("sqlite3", "db.sqlite")
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer db.Close()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	server := NewServer(db, logger)
	err = server.Run(context.Background(), "127.0.0.1:5000")
	if err != nil {
		logger.Fatalf("error running server: %v\n", err)
	}
}
