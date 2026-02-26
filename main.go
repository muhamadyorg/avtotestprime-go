package main

import (
        "encoding/json"
        "fmt"
        "html/template"
        "io"
        "log"
        "math"
        "mime/multipart"
        "net/http"
        "os"
        "path/filepath"
        "strings"
        "time"

        "github.com/gorilla/mux"
        "github.com/gorilla/sessions"
)

var (
        store     *sessions.CookieStore
        templates map[string]*template.Template
)

var funcMap = template.FuncMap{
        "add": func(a, b int) int { return a + b },
        "sub": func(a, b int) int { return a - b },
        "mul": func(a, b int) int { return a * b },
        "contains": func(set map[int]bool, id int) bool {
                if set == nil {
                        return false
                }
                return set[id]
        },
        "lower": strings.ToLower,
        "formatDate": func(t time.Time, format string) string {
                switch format {
                case "d.m.Y H:i":
                        return t.Format("02.01.2006 15:04")
                case "d.m.Y":
                        return t.Format("02.01.2006")
                default:
                        return t.Format("02.01.2006 15:04")
                }
        },
        "truncateWords": func(s string, n int) string {
                words := strings.Fields(s)
                if len(words) <= n {
                        return s
                }
                return strings.Join(words[:n], " ") + " ..."
        },
        "scoreClass": func(percent int) string {
                if percent >= 80 {
                        return "score-good"
                } else if percent >= 50 {
                        return "score-ok"
                }
                return "score-bad"
        },
        "toJSON": func(v interface{}) template.JS {
                b, _ := json.Marshal(v)
                return template.JS(b)
        },
        "imageURL": func(path string) string {
                if path == "" {
                        return ""
                }
                return "/media/" + path
        },
        "hasImage": func(path string) bool {
                return path != ""
        },
        "strContains": strings.Contains,
        "roundFloat": func(f float64) int {
                return int(math.Round(f))
        },
}

func loadTemplates() {
        templates = make(map[string]*template.Template)
        base := "templates/base.html"
        pages := []string{
                "dashboard.html",
                "all_questions.html",
                "question_detail.html",
                "search.html",
                "bookmarks.html",
                "start_test.html",
                "take_test.html",
                "test_result.html",
                "statistics.html",
                "profile.html",
                "admin/dashboard.html",
                "admin/questions.html",
                "admin/add_question.html",
                "admin/edit_question.html",
                "admin/users.html",
                "admin/add_user.html",
                "admin/edit_user.html",
                "admin/statistics.html",
        }
        for _, page := range pages {
                t := template.Must(template.New("").Funcs(funcMap).ParseFiles(base, "templates/"+page))
                templates[page] = t
        }
        templates["login.html"] = template.Must(template.New("login.html").Funcs(funcMap).ParseFiles("templates/login.html"))
}

func renderTemplate(w http.ResponseWriter, r *http.Request, tmplName string, data map[string]interface{}) {
        if data == nil {
                data = make(map[string]interface{})
        }
        user := getCurrentUser(r)
        if user != nil {
                data["User"] = user
                data["IsAuthenticated"] = true
                data["IsStaff"] = user.IsStaff
        } else {
                data["IsAuthenticated"] = false
                data["IsStaff"] = false
        }
        data["CSRFToken"] = getCSRFToken(w, r)

        tmpl, ok := templates[tmplName]
        if !ok {
                log.Printf("Template not found: %s", tmplName)
                http.Error(w, "Template not found", 500)
                return
        }

        if tmplName == "login.html" {
                err := tmpl.Execute(w, data)
                if err != nil {
                        log.Printf("Template error (%s): %v", tmplName, err)
                }
        } else {
                err := tmpl.ExecuteTemplate(w, "base", data)
                if err != nil {
                        log.Printf("Template error (%s): %v", tmplName, err)
                }
        }
}

func getCSRFToken(w http.ResponseWriter, r *http.Request) string {
        session, _ := store.Get(r, "session")
        token, ok := session.Values["csrf_token"].(string)
        if !ok || token == "" {
                token = generateRandomString(32)
                session.Values["csrf_token"] = token
                session.Save(r, w)
        }
        return token
}

func verifyCSRFToken(r *http.Request, w http.ResponseWriter) bool {
        session, _ := store.Get(r, "session")
        expected, ok := session.Values["csrf_token"].(string)
        if !ok || expected == "" {
                return false
        }
        provided := r.FormValue("csrf_token")
        return provided == expected
}

func saveUploadedFile(file multipart.File, header *multipart.FileHeader) (string, error) {
        os.MkdirAll("media/questions", 0755)
        ext := filepath.Ext(header.Filename)
        filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
        path := filepath.Join("media", "questions", filename)
        dst, err := os.Create(path)
        if err != nil {
                return "", err
        }
        defer dst.Close()
        _, err = io.Copy(dst, file)
        return "questions/" + filename, err
}

func main() {
        secret := os.Getenv("SESSION_SECRET")
        if secret == "" {
                secret = "default-secret-key-change-in-production"
                log.Println("WARNING: SESSION_SECRET not set, using insecure default")
        }
        store = sessions.NewCookieStore([]byte(secret))
        store.Options = &sessions.Options{
                Path:     "/",
                MaxAge:   86400 * 30,
                HttpOnly: true,
        }

        initDB()
        defer db.Close()

        loadTemplates()

        r := mux.NewRouter()

        r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
        r.PathPrefix("/media/").Handler(http.StripPrefix("/media/", http.FileServer(http.Dir("media"))))

        r.HandleFunc("/", indexHandler)
        r.HandleFunc("/login/", loginHandler)
        r.HandleFunc("/logout/", logoutHandler)

        r.HandleFunc("/dashboard/", authRequired(dashboardHandler))
        r.HandleFunc("/questions/", authRequired(allQuestionsHandler))
        r.HandleFunc("/questions/{id}/", authRequired(questionDetailHandler))
        r.HandleFunc("/search/", authRequired(searchQuestionsHandler))
        r.HandleFunc("/bookmark/toggle/{id}/", authRequired(toggleBookmarkHandler))
        r.HandleFunc("/bookmarks/", authRequired(bookmarksHandler))
        r.HandleFunc("/test/start/", authRequired(startTestHandler))
        r.HandleFunc("/test/{id}/", authRequired(takeTestHandler))
        r.HandleFunc("/test/{id}/submit/", authRequired(submitTestHandler))
        r.HandleFunc("/test/{id}/result/", authRequired(testResultHandler))
        r.HandleFunc("/statistics/", authRequired(statisticsHandler))
        r.HandleFunc("/profile/", authRequired(profileHandler))

        r.HandleFunc("/admin-panel/", adminRequired(adminDashboardHandler))
        r.HandleFunc("/admin-panel/questions/", adminRequired(adminQuestionsHandler))
        r.HandleFunc("/admin-panel/questions/add/", adminRequired(adminAddQuestionHandler))
        r.HandleFunc("/admin-panel/questions/{id}/edit/", adminRequired(adminEditQuestionHandler))
        r.HandleFunc("/admin-panel/questions/{id}/delete/", adminRequired(adminDeleteQuestionHandler))
        r.HandleFunc("/admin-panel/users/", adminRequired(adminUsersHandler))
        r.HandleFunc("/admin-panel/users/add/", adminRequired(adminAddUserHandler))
        r.HandleFunc("/admin-panel/users/{id}/edit/", adminRequired(adminEditUserHandler))
        r.HandleFunc("/admin-panel/users/{id}/delete/", adminRequired(adminDeleteUserHandler))
        r.HandleFunc("/admin-panel/statistics/", adminRequired(adminStatisticsHandler))

        port := os.Getenv("PORT")
        if port == "" {
                port = "5000"
        }

        log.Printf("AvtotestPrime starting on :%s", port)
        log.Fatal(http.ListenAndServe("0.0.0.0:"+port, recoveryMiddleware(r)))
}
