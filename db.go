package main

import (
        "database/sql"
        "log"
        "os"

        _ "github.com/lib/pq"
        "golang.org/x/crypto/bcrypt"
)

var db *sql.DB

func initDB() {
        dsn := os.Getenv("DATABASE_URL")
        if dsn == "" {
                log.Fatal("DATABASE_URL environment variable is required")
        }

        var err error
        db, err = sql.Open("postgres", dsn)
        if err != nil {
                log.Fatalf("Failed to connect to database: %v", err)
        }

        if err = db.Ping(); err != nil {
                log.Fatalf("Failed to ping database: %v", err)
        }

        log.Println("Database connected successfully")
        migrate()
        seedDefaultUsers()
}

func migrate() {
        queries := []string{
                `CREATE TABLE IF NOT EXISTS users (
                        id SERIAL PRIMARY KEY,
                        username VARCHAR(255) UNIQUE NOT NULL,
                        password_hash VARCHAR(255) NOT NULL,
                        is_staff BOOLEAN DEFAULT FALSE,
                        date_joined TIMESTAMP DEFAULT NOW()
                )`,
                `CREATE TABLE IF NOT EXISTS questions (
                        id SERIAL PRIMARY KEY,
                        number INTEGER UNIQUE NOT NULL,
                        text TEXT NOT NULL,
                        image VARCHAR(500) DEFAULT '',
                        variants_json TEXT DEFAULT '[]',
                        correct_answer VARCHAR(1) NOT NULL,
                        variant_a VARCHAR(500) DEFAULT '',
                        variant_b VARCHAR(500) DEFAULT '',
                        variant_c VARCHAR(500) DEFAULT '',
                        variant_d VARCHAR(500) DEFAULT '',
                        created_at TIMESTAMP DEFAULT NOW(),
                        updated_at TIMESTAMP DEFAULT NOW()
                )`,
                `CREATE TABLE IF NOT EXISTS bookmarks (
                        id SERIAL PRIMARY KEY,
                        user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
                        question_id INTEGER REFERENCES questions(id) ON DELETE CASCADE,
                        created_at TIMESTAMP DEFAULT NOW(),
                        UNIQUE(user_id, question_id)
                )`,
                `CREATE TABLE IF NOT EXISTS test_sessions (
                        id SERIAL PRIMARY KEY,
                        user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
                        total_questions INTEGER NOT NULL,
                        correct_answers INTEGER DEFAULT 0,
                        wrong_answers INTEGER DEFAULT 0,
                        time_spent INTEGER DEFAULT 0,
                        completed BOOLEAN DEFAULT FALSE,
                        question_ids TEXT DEFAULT '[]',
                        created_at TIMESTAMP DEFAULT NOW()
                )`,
                `CREATE TABLE IF NOT EXISTS test_answers (
                        id SERIAL PRIMARY KEY,
                        session_id INTEGER REFERENCES test_sessions(id) ON DELETE CASCADE,
                        question_id INTEGER REFERENCES questions(id) ON DELETE CASCADE,
                        selected_answer VARCHAR(1) DEFAULT '',
                        is_correct BOOLEAN DEFAULT FALSE
                )`,
        }

        for _, q := range queries {
                _, err := db.Exec(q)
                if err != nil {
                        log.Printf("Migration error: %v\nQuery: %s", err, q)
                }
        }

        log.Println("Database migrations completed")
}

func seedDefaultUsers() {
        adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
        userHash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)

        var adminExists bool
        db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username='admin')").Scan(&adminExists)
        if adminExists {
                var hash string
                db.QueryRow("SELECT password_hash FROM users WHERE username='admin'").Scan(&hash)
                if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin")); err != nil {
                        db.Exec("UPDATE users SET password_hash=$1 WHERE username='admin'", string(adminHash))
                        log.Println("Admin password updated to bcrypt format")
                }
        } else {
                db.Exec("INSERT INTO users (username, password_hash, is_staff) VALUES ($1, $2, $3)",
                        "admin", string(adminHash), true)
                log.Println("Admin user created (admin/admin)")
        }

        var userExists bool
        db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username='user')").Scan(&userExists)
        if userExists {
                var hash string
                db.QueryRow("SELECT password_hash FROM users WHERE username='user'").Scan(&hash)
                if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("user")); err != nil {
                        db.Exec("UPDATE users SET password_hash=$1 WHERE username='user'", string(userHash))
                        log.Println("User password updated to bcrypt format")
                }
        } else {
                db.Exec("INSERT INTO users (username, password_hash, is_staff) VALUES ($1, $2, $3)",
                        "user", string(userHash), false)
                log.Println("Default user created (user/user)")
        }

        log.Println("Default users ready (admin/admin, user/user)")
}
