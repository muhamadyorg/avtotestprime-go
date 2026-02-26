package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID         int
	Username   string
	PassHash   string
	IsStaff    bool
	DateJoined time.Time
}

type Variant struct {
	Letter string `json:"letter"`
	Text   string `json:"text"`
}

type Question struct {
	ID            int
	Number        int
	Text          string
	Image         string
	VariantsJSON  string
	CorrectAnswer string
	VariantA      string
	VariantB      string
	VariantC      string
	VariantD      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	VariantsList  []Variant
}

type Bookmark struct {
	ID         int
	UserID     int
	QuestionID int
	CreatedAt  time.Time
}

type TestSession struct {
	ID              int
	UserID          int
	TotalQuestions  int
	CorrectAnswers  int
	WrongAnswers    int
	TimeSpent       int
	Completed       bool
	QuestionIDs     string
	CreatedAt       time.Time
	Username        string
	ScorePercent    int
}

type TestAnswer struct {
	ID             int
	SessionID      int
	QuestionID     int
	SelectedAnswer string
	IsCorrect      bool
	Question       *Question
}

func generateRandomString(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:n]
}

func (q *Question) ComputeVariants() {
	var variants []Variant
	if q.VariantsJSON != "" && q.VariantsJSON != "[]" {
		json.Unmarshal([]byte(q.VariantsJSON), &variants)
	}
	if len(variants) == 0 {
		pairs := []struct {
			field  string
			letter string
		}{
			{q.VariantA, "A"}, {q.VariantB, "B"}, {q.VariantC, "C"}, {q.VariantD, "D"},
		}
		for _, p := range pairs {
			if p.field != "" {
				variants = append(variants, Variant{Letter: p.letter, Text: p.field})
			}
		}
	}
	q.VariantsList = variants
}

func (s *TestSession) CalcScorePercent() {
	if s.TotalQuestions == 0 {
		s.ScorePercent = 0
		return
	}
	s.ScorePercent = int(float64(s.CorrectAnswers) / float64(s.TotalQuestions) * 100)
}

func getUserByID(id int) *User {
	u := &User{}
	err := db.QueryRow("SELECT id, username, password_hash, is_staff, date_joined FROM users WHERE id=$1", id).
		Scan(&u.ID, &u.Username, &u.PassHash, &u.IsStaff, &u.DateJoined)
	if err != nil {
		return nil
	}
	return u
}

func getUserByUsername(username string) *User {
	u := &User{}
	err := db.QueryRow("SELECT id, username, password_hash, is_staff, date_joined FROM users WHERE username=$1", username).
		Scan(&u.ID, &u.Username, &u.PassHash, &u.IsStaff, &u.DateJoined)
	if err != nil {
		return nil
	}
	return u
}

func authenticateUser(username, password string) *User {
	u := getUserByUsername(username)
	if u == nil {
		return nil
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PassHash), []byte(password))
	if err != nil {
		return nil
	}
	return u
}

func createUser(username, password string, isStaff bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO users (username, password_hash, is_staff) VALUES ($1, $2, $3)",
		username, string(hash), isStaff)
	return err
}

func updateUserUsername(id int, username string) error {
	_, err := db.Exec("UPDATE users SET username=$1 WHERE id=$2", username, id)
	return err
}

func updateUserPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE users SET password_hash=$1 WHERE id=$2", string(hash), id)
	return err
}

func getNonStaffUsers() []*User {
	rows, err := db.Query("SELECT id, username, password_hash, is_staff, date_joined FROM users WHERE is_staff=FALSE ORDER BY id")
	if err != nil {
		log.Printf("Error getting users: %v", err)
		return nil
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		rows.Scan(&u.ID, &u.Username, &u.PassHash, &u.IsStaff, &u.DateJoined)
		users = append(users, u)
	}
	return users
}

func countNonStaffUsers() int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE is_staff=FALSE").Scan(&count)
	return count
}

func deleteUser(id int) error {
	_, err := db.Exec("DELETE FROM users WHERE id=$1 AND is_staff=FALSE", id)
	return err
}

func usernameExists(username string, excludeID int) bool {
	var count int
	if excludeID > 0 {
		db.QueryRow("SELECT COUNT(*) FROM users WHERE username=$1 AND id!=$2", username, excludeID).Scan(&count)
	} else {
		db.QueryRow("SELECT COUNT(*) FROM users WHERE username=$1", username).Scan(&count)
	}
	return count > 0
}

func scanQuestion(row interface{ Scan(...interface{}) error }) *Question {
	q := &Question{}
	var image sql.NullString
	err := row.Scan(&q.ID, &q.Number, &q.Text, &image, &q.VariantsJSON, &q.CorrectAnswer,
		&q.VariantA, &q.VariantB, &q.VariantC, &q.VariantD, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil
	}
	if image.Valid {
		q.Image = image.String
	}
	q.ComputeVariants()
	return q
}

func getAllQuestions() []*Question {
	rows, err := db.Query("SELECT id, number, text, image, variants_json, correct_answer, variant_a, variant_b, variant_c, variant_d, created_at, updated_at FROM questions ORDER BY number")
	if err != nil {
		log.Printf("Error getting questions: %v", err)
		return nil
	}
	defer rows.Close()
	var questions []*Question
	for rows.Next() {
		q := &Question{}
		var image sql.NullString
		rows.Scan(&q.ID, &q.Number, &q.Text, &image, &q.VariantsJSON, &q.CorrectAnswer,
			&q.VariantA, &q.VariantB, &q.VariantC, &q.VariantD, &q.CreatedAt, &q.UpdatedAt)
		if image.Valid {
			q.Image = image.String
		}
		q.ComputeVariants()
		questions = append(questions, q)
	}
	return questions
}

func getQuestionByID(id int) *Question {
	row := db.QueryRow("SELECT id, number, text, image, variants_json, correct_answer, variant_a, variant_b, variant_c, variant_d, created_at, updated_at FROM questions WHERE id=$1", id)
	return scanQuestion(row)
}

func countQuestions() int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM questions").Scan(&count)
	return count
}

func getNextQuestionNumber() int {
	var maxNum sql.NullInt64
	db.QueryRow("SELECT MAX(number) FROM questions").Scan(&maxNum)
	if maxNum.Valid {
		return int(maxNum.Int64) + 1
	}
	return 1
}

func createQuestion(q *Question) error {
	varJSON, _ := json.Marshal(q.VariantsList)
	_, err := db.Exec(`INSERT INTO questions (number, text, image, variants_json, correct_answer, variant_a, variant_b, variant_c, variant_d) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		q.Number, q.Text, q.Image, string(varJSON), q.CorrectAnswer,
		q.VariantA, q.VariantB, q.VariantC, q.VariantD)
	return err
}

func updateQuestion(q *Question) error {
	varJSON, _ := json.Marshal(q.VariantsList)
	_, err := db.Exec(`UPDATE questions SET text=$1, image=$2, variants_json=$3, correct_answer=$4, 
		variant_a=$5, variant_b=$6, variant_c=$7, variant_d=$8, updated_at=NOW() WHERE id=$9`,
		q.Text, q.Image, string(varJSON), q.CorrectAnswer,
		q.VariantA, q.VariantB, q.VariantC, q.VariantD, q.ID)
	return err
}

func deleteQuestion(id int) error {
	_, err := db.Exec("DELETE FROM questions WHERE id=$1", id)
	return err
}

func searchQuestions(query string) []*Question {
	searchPattern := "%" + strings.ToLower(query) + "%"
	rows, err := db.Query(`SELECT id, number, text, image, variants_json, correct_answer, 
		variant_a, variant_b, variant_c, variant_d, created_at, updated_at 
		FROM questions WHERE LOWER(text) LIKE $1 OR LOWER(variant_a) LIKE $1 OR LOWER(variant_b) LIKE $1 
		OR LOWER(variant_c) LIKE $1 OR LOWER(variant_d) LIKE $1 OR CAST(number AS TEXT) LIKE $1
		ORDER BY number`, searchPattern)
	if err != nil {
		log.Printf("Error searching questions: %v", err)
		return nil
	}
	defer rows.Close()
	var questions []*Question
	for rows.Next() {
		q := &Question{}
		var image sql.NullString
		rows.Scan(&q.ID, &q.Number, &q.Text, &image, &q.VariantsJSON, &q.CorrectAnswer,
			&q.VariantA, &q.VariantB, &q.VariantC, &q.VariantD, &q.CreatedAt, &q.UpdatedAt)
		if image.Valid {
			q.Image = image.String
		}
		q.ComputeVariants()
		questions = append(questions, q)
	}
	return questions
}

func getQuestionsByIDs(ids []int) map[int]*Question {
	if len(ids) == 0 {
		return make(map[int]*Question)
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT id, number, text, image, variants_json, correct_answer, 
		variant_a, variant_b, variant_c, variant_d, created_at, updated_at 
		FROM questions WHERE id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error getting questions by IDs: %v", err)
		return make(map[int]*Question)
	}
	defer rows.Close()
	result := make(map[int]*Question)
	for rows.Next() {
		q := &Question{}
		var image sql.NullString
		rows.Scan(&q.ID, &q.Number, &q.Text, &image, &q.VariantsJSON, &q.CorrectAnswer,
			&q.VariantA, &q.VariantB, &q.VariantC, &q.VariantD, &q.CreatedAt, &q.UpdatedAt)
		if image.Valid {
			q.Image = image.String
		}
		q.ComputeVariants()
		result[q.ID] = q
	}
	return result
}

func getRandomQuestionIDs(limit int) []int {
	rows, err := db.Query("SELECT id FROM questions ORDER BY RANDOM() LIMIT $1", limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

func getUserBookmarkIDs(userID int) map[int]bool {
	rows, err := db.Query("SELECT question_id FROM bookmarks WHERE user_id=$1", userID)
	if err != nil {
		return make(map[int]bool)
	}
	defer rows.Close()
	result := make(map[int]bool)
	for rows.Next() {
		var qid int
		rows.Scan(&qid)
		result[qid] = true
	}
	return result
}

func toggleBookmark(userID, questionID int) string {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE user_id=$1 AND question_id=$2", userID, questionID).Scan(&count)
	if count > 0 {
		db.Exec("DELETE FROM bookmarks WHERE user_id=$1 AND question_id=$2", userID, questionID)
		return "removed"
	}
	db.Exec("INSERT INTO bookmarks (user_id, question_id) VALUES ($1, $2)", userID, questionID)
	return "added"
}

func getBookmarkedQuestions(userID int) []*Question {
	rows, err := db.Query(`SELECT q.id, q.number, q.text, q.image, q.variants_json, q.correct_answer, 
		q.variant_a, q.variant_b, q.variant_c, q.variant_d, q.created_at, q.updated_at 
		FROM questions q JOIN bookmarks b ON q.id = b.question_id 
		WHERE b.user_id=$1 ORDER BY q.number`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var questions []*Question
	for rows.Next() {
		q := &Question{}
		var image sql.NullString
		rows.Scan(&q.ID, &q.Number, &q.Text, &image, &q.VariantsJSON, &q.CorrectAnswer,
			&q.VariantA, &q.VariantB, &q.VariantC, &q.VariantD, &q.CreatedAt, &q.UpdatedAt)
		if image.Valid {
			q.Image = image.String
		}
		q.ComputeVariants()
		questions = append(questions, q)
	}
	return questions
}

func countBookmarks(userID int) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE user_id=$1", userID).Scan(&count)
	return count
}

func createTestSession(userID, totalQuestions int, questionIDs []int) (*TestSession, error) {
	idsJSON, _ := json.Marshal(questionIDs)
	var id int
	err := db.QueryRow(`INSERT INTO test_sessions (user_id, total_questions, question_ids) 
		VALUES ($1, $2, $3) RETURNING id`, userID, totalQuestions, string(idsJSON)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &TestSession{
		ID:             id,
		UserID:         userID,
		TotalQuestions: totalQuestions,
		QuestionIDs:    string(idsJSON),
	}, nil
}

func getTestSession(id, userID int) *TestSession {
	s := &TestSession{}
	err := db.QueryRow(`SELECT id, user_id, total_questions, correct_answers, wrong_answers, 
		time_spent, completed, question_ids, created_at FROM test_sessions WHERE id=$1 AND user_id=$2`,
		id, userID).Scan(&s.ID, &s.UserID, &s.TotalQuestions, &s.CorrectAnswers, &s.WrongAnswers,
		&s.TimeSpent, &s.Completed, &s.QuestionIDs, &s.CreatedAt)
	if err != nil {
		return nil
	}
	s.CalcScorePercent()
	return s
}

func updateTestSession(s *TestSession) error {
	_, err := db.Exec(`UPDATE test_sessions SET correct_answers=$1, wrong_answers=$2, time_spent=$3, completed=$4 WHERE id=$5`,
		s.CorrectAnswers, s.WrongAnswers, s.TimeSpent, s.Completed, s.ID)
	return err
}

func getUserCompletedSessions(userID int) []*TestSession {
	rows, err := db.Query(`SELECT id, user_id, total_questions, correct_answers, wrong_answers, 
		time_spent, completed, question_ids, created_at FROM test_sessions 
		WHERE user_id=$1 AND completed=TRUE ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var sessions []*TestSession
	for rows.Next() {
		s := &TestSession{}
		rows.Scan(&s.ID, &s.UserID, &s.TotalQuestions, &s.CorrectAnswers, &s.WrongAnswers,
			&s.TimeSpent, &s.Completed, &s.QuestionIDs, &s.CreatedAt)
		s.CalcScorePercent()
		sessions = append(sessions, s)
	}
	return sessions
}

func countUserCompletedSessions(userID int) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM test_sessions WHERE user_id=$1 AND completed=TRUE", userID).Scan(&count)
	return count
}

func getRecentCompletedSessions(limit int) []*TestSession {
	rows, err := db.Query(`SELECT ts.id, ts.user_id, ts.total_questions, ts.correct_answers, ts.wrong_answers, 
		ts.time_spent, ts.completed, ts.question_ids, ts.created_at, u.username 
		FROM test_sessions ts JOIN users u ON ts.user_id = u.id 
		WHERE ts.completed=TRUE ORDER BY ts.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var sessions []*TestSession
	for rows.Next() {
		s := &TestSession{}
		rows.Scan(&s.ID, &s.UserID, &s.TotalQuestions, &s.CorrectAnswers, &s.WrongAnswers,
			&s.TimeSpent, &s.Completed, &s.QuestionIDs, &s.CreatedAt, &s.Username)
		s.CalcScorePercent()
		sessions = append(sessions, s)
	}
	return sessions
}

func countCompletedSessions() int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM test_sessions WHERE completed=TRUE").Scan(&count)
	return count
}

func createTestAnswer(sessionID, questionID int, selectedAnswer string, isCorrect bool) error {
	_, err := db.Exec("INSERT INTO test_answers (session_id, question_id, selected_answer, is_correct) VALUES ($1, $2, $3, $4)",
		sessionID, questionID, selectedAnswer, isCorrect)
	return err
}

func getSessionAnswers(sessionID int) []*TestAnswer {
	rows, err := db.Query(`SELECT ta.id, ta.session_id, ta.question_id, ta.selected_answer, ta.is_correct,
		q.id, q.number, q.text, q.image, q.variants_json, q.correct_answer, 
		q.variant_a, q.variant_b, q.variant_c, q.variant_d, q.created_at, q.updated_at
		FROM test_answers ta JOIN questions q ON ta.question_id = q.id 
		WHERE ta.session_id=$1 ORDER BY ta.id`, sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var answers []*TestAnswer
	for rows.Next() {
		a := &TestAnswer{Question: &Question{}}
		var image sql.NullString
		rows.Scan(&a.ID, &a.SessionID, &a.QuestionID, &a.SelectedAnswer, &a.IsCorrect,
			&a.Question.ID, &a.Question.Number, &a.Question.Text, &image, &a.Question.VariantsJSON,
			&a.Question.CorrectAnswer, &a.Question.VariantA, &a.Question.VariantB,
			&a.Question.VariantC, &a.Question.VariantD, &a.Question.CreatedAt, &a.Question.UpdatedAt)
		if image.Valid {
			a.Question.Image = image.String
		}
		a.Question.ComputeVariants()
		answers = append(answers, a)
	}
	return answers
}

func getRandomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}
