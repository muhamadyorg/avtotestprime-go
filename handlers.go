package main

import (
        "encoding/json"
        "fmt"
        "net/http"
        "strconv"
        "strings"

        "github.com/gorilla/mux"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        if user == nil {
                http.Redirect(w, r, "/login/", http.StatusFound)
                return
        }
        if user.IsStaff {
                http.Redirect(w, r, "/admin-panel/", http.StatusFound)
        } else {
                http.Redirect(w, r, "/dashboard/", http.StatusFound)
        }
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        if user != nil {
                if user.IsStaff {
                        http.Redirect(w, r, "/admin-panel/", http.StatusFound)
                } else {
                        http.Redirect(w, r, "/dashboard/", http.StatusFound)
                }
                return
        }

        data := map[string]interface{}{}

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        data["Error"] = "Xavfsizlik xatosi. Qayta urinib ko'ring."
                        renderTemplate(w, r, "login.html", data)
                        return
                }
                username := r.FormValue("username")
                password := r.FormValue("password")
                u := authenticateUser(username, password)
                if u != nil {
                        setCurrentUser(w, r, u.ID)
                        if u.IsStaff {
                                http.Redirect(w, r, "/admin-panel/", http.StatusFound)
                        } else {
                                http.Redirect(w, r, "/dashboard/", http.StatusFound)
                        }
                        return
                }
                data["Error"] = "Login yoki parol xato!"
        }

        renderTemplate(w, r, "login.html", data)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
        clearCurrentUser(w, r)
        http.Redirect(w, r, "/login/", http.StatusFound)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        totalQuestions := countQuestions()
        bookmarkCount := countBookmarks(user.ID)
        testCount := countUserCompletedSessions(user.ID)

        avgScore := 0
        sessions := getUserCompletedSessions(user.ID)
        if len(sessions) > 0 {
                total := 0
                for _, s := range sessions {
                        total += s.ScorePercent
                }
                avgScore = total / len(sessions)
        }

        renderTemplate(w, r, "dashboard.html", map[string]interface{}{
                "CurrentPage":    "dashboard",
                "TotalQuestions": totalQuestions,
                "BookmarkCount":  bookmarkCount,
                "TestCount":      testCount,
                "AvgScore":       avgScore,
        })
}

func allQuestionsHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        questions := getAllQuestions()
        userBookmarks := getUserBookmarkIDs(user.ID)

        renderTemplate(w, r, "all_questions.html", map[string]interface{}{
                "CurrentPage":   "all_questions",
                "Questions":     questions,
                "UserBookmarks": userBookmarks,
        })
}

func questionDetailHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        question := getQuestionByID(id)
        if question == nil {
                http.NotFound(w, r)
                return
        }
        bookmarks := getUserBookmarkIDs(user.ID)

        renderTemplate(w, r, "question_detail.html", map[string]interface{}{
                "CurrentPage":  "all_questions",
                "QuestionData": question,
                "IsBookmarked": bookmarks[question.ID],
        })
}

func searchQuestionsHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        query := r.URL.Query().Get("q")
        var questions []*Question
        if query != "" {
                questions = searchQuestions(query)
        } else {
                questions = []*Question{}
        }
        userBookmarks := getUserBookmarkIDs(user.ID)

        renderTemplate(w, r, "search.html", map[string]interface{}{
                "CurrentPage":   "search_questions",
                "Questions":     questions,
                "Query":         query,
                "UserBookmarks": userBookmarks,
        })
}

func toggleBookmarkHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        status := toggleBookmark(user.ID, id)

        if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
                w.Header().Set("Content-Type", "application/json")
                json.NewEncoder(w).Encode(map[string]string{"status": status})
                return
        }
        referer := r.Header.Get("Referer")
        if referer == "" {
                referer = "/questions/"
        }
        http.Redirect(w, r, referer, http.StatusFound)
}

func bookmarksHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        questions := getBookmarkedQuestions(user.ID)

        renderTemplate(w, r, "bookmarks.html", map[string]interface{}{
                "CurrentPage": "bookmarks",
                "Questions":   questions,
        })
}

func startTestHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        totalAvailable := countQuestions()

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                numQuestions, _ := strconv.Atoi(r.FormValue("num_questions"))
                if numQuestions < 1 {
                        numQuestions = 1
                }
                if numQuestions > totalAvailable {
                        numQuestions = totalAvailable
                }
                questionIDs := getRandomQuestionIDs(numQuestions)
                session, err := createTestSession(user.ID, numQuestions, questionIDs)
                if err != nil {
                        http.Error(w, "Error creating test session", 500)
                        return
                }
                http.Redirect(w, r, fmt.Sprintf("/test/%d/", session.ID), http.StatusFound)
                return
        }

        renderTemplate(w, r, "start_test.html", map[string]interface{}{
                "CurrentPage":    "start_test",
                "TotalAvailable": totalAvailable,
        })
}

func takeTestHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        session := getTestSession(id, user.ID)
        if session == nil {
                http.Redirect(w, r, "/test/start/", http.StatusFound)
                return
        }
        if session.Completed {
                http.Redirect(w, r, fmt.Sprintf("/test/%d/result/", session.ID), http.StatusFound)
                return
        }

        var questionIDs []int
        json.Unmarshal([]byte(session.QuestionIDs), &questionIDs)
        qMap := getQuestionsByIDs(questionIDs)
        var orderedQuestions []*Question
        for _, qid := range questionIDs {
                if q, ok := qMap[qid]; ok {
                        orderedQuestions = append(orderedQuestions, q)
                }
        }

        timeLimit := session.TotalQuestions * 60

        renderTemplate(w, r, "take_test.html", map[string]interface{}{
                "CurrentPage":    "start_test",
                "Session":        session,
                "Questions":      orderedQuestions,
                "TimeLimit":      timeLimit,
                "TotalQuestions": len(orderedQuestions),
        })
}

func submitTestHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        session := getTestSession(id, user.ID)
        if session == nil {
                http.Redirect(w, r, "/test/start/", http.StatusFound)
                return
        }
        if session.Completed {
                http.Redirect(w, r, fmt.Sprintf("/test/%d/result/", session.ID), http.StatusFound)
                return
        }

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                timeSpent, _ := strconv.Atoi(r.FormValue("time_spent"))

                var questionIDs []int
                json.Unmarshal([]byte(session.QuestionIDs), &questionIDs)
                qMap := getQuestionsByIDs(questionIDs)

                correct := 0
                wrong := 0
                for _, qid := range questionIDs {
                        answer := strings.ToUpper(r.FormValue(fmt.Sprintf("answer_%d", qid)))
                        q, ok := qMap[qid]
                        if !ok {
                                continue
                        }
                        if answer != "" {
                                isCorrect := answer == q.CorrectAnswer
                                if isCorrect {
                                        correct++
                                } else {
                                        wrong++
                                }
                                createTestAnswer(session.ID, qid, answer, isCorrect)
                        } else {
                                wrong++
                                createTestAnswer(session.ID, qid, "", false)
                        }
                }

                session.CorrectAnswers = correct
                session.WrongAnswers = wrong
                session.TimeSpent = timeSpent
                session.Completed = true
                updateTestSession(session)

                http.Redirect(w, r, fmt.Sprintf("/test/%d/result/", session.ID), http.StatusFound)
                return
        }

        http.Redirect(w, r, fmt.Sprintf("/test/%d/", session.ID), http.StatusFound)
}

func testResultHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        session := getTestSession(id, user.ID)
        if session == nil {
                http.Redirect(w, r, "/test/start/", http.StatusFound)
                return
        }
        answers := getSessionAnswers(session.ID)

        renderTemplate(w, r, "test_result.html", map[string]interface{}{
                "CurrentPage": "start_test",
                "Session":     session,
                "Answers":     answers,
        })
}

func statisticsHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        sessions := getUserCompletedSessions(user.ID)

        totalTests := len(sessions)
        avgScore := 0
        bestScore := 0
        totalCorrect := 0

        if totalTests > 0 {
                totalScore := 0
                for _, s := range sessions {
                        totalScore += s.ScorePercent
                        if s.ScorePercent > bestScore {
                                bestScore = s.ScorePercent
                        }
                        totalCorrect += s.CorrectAnswers
                }
                avgScore = totalScore / totalTests
        }

        var recentSessions []*TestSession
        if len(sessions) > 10 {
                recentSessions = sessions[:10]
        } else {
                recentSessions = sessions
        }

        renderTemplate(w, r, "statistics.html", map[string]interface{}{
                "CurrentPage":    "statistics",
                "TotalTests":     totalTests,
                "AvgScore":       avgScore,
                "BestScore":      bestScore,
                "TotalCorrect":   totalCorrect,
                "RecentSessions": recentSessions,
        })
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
        user := getCurrentUser(r)
        data := map[string]interface{}{
                "CurrentPage": "profile",
        }

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                newUsername := strings.TrimSpace(r.FormValue("new_username"))
                newPassword := strings.TrimSpace(r.FormValue("new_password"))
                changed := false

                if newUsername != "" && newUsername != user.Username {
                        if usernameExists(newUsername, user.ID) {
                                data["Error"] = "Bu login allaqachon mavjud!"
                        } else {
                                updateUserUsername(user.ID, newUsername)
                                changed = true
                        }
                }

                if newPassword != "" && data["Error"] == nil {
                        updateUserPassword(user.ID, newPassword)
                        changed = true
                }

                if changed && data["Error"] == nil {
                        data["Success"] = "Ma'lumotlar muvaffaqiyatli yangilandi!"
                        user = getUserByID(user.ID)
                }
        }

        renderTemplate(w, r, "profile.html", data)
}

func adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
        totalQuestions := countQuestions()
        totalUsers := countNonStaffUsers()
        totalTests := countCompletedSessions()
        recentTests := getRecentCompletedSessions(5)

        renderTemplate(w, r, "admin/dashboard.html", map[string]interface{}{
                "CurrentPage":    "admin_dashboard",
                "TotalQuestions": totalQuestions,
                "TotalUsers":     totalUsers,
                "TotalTests":     totalTests,
                "RecentTests":    recentTests,
        })
}

func adminQuestionsHandler(w http.ResponseWriter, r *http.Request) {
        questions := getAllQuestions()
        renderTemplate(w, r, "admin/questions.html", map[string]interface{}{
                "CurrentPage": "admin_questions",
                "Questions":   questions,
        })
}

func parseVariantsFromForm(r *http.Request) []Variant {
        letters := "ABCDEFGHIJ"
        var variants []Variant
        for _, letter := range letters {
                l := string(letter)
                val := strings.TrimSpace(r.FormValue("variant_" + strings.ToLower(l)))
                if val != "" {
                        variants = append(variants, Variant{Letter: l, Text: val})
                }
        }
        return variants
}

func adminAddQuestionHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
                r.ParseMultipartForm(10 << 20)
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                nextNum := getNextQuestionNumber()
                variants := parseVariantsFromForm(r)

                q := &Question{
                        Number:        nextNum,
                        Text:          r.FormValue("text"),
                        CorrectAnswer: r.FormValue("correct_answer"),
                        VariantsList:  variants,
                }

                file, header, err := r.FormFile("image")
                if err == nil {
                        defer file.Close()
                        imagePath, err := saveUploadedFile(file, header)
                        if err == nil {
                                q.Image = imagePath
                        }
                }

                createQuestion(q)
                http.Redirect(w, r, "/admin-panel/questions/", http.StatusFound)
                return
        }

        renderTemplate(w, r, "admin/add_question.html", map[string]interface{}{
                "CurrentPage": "admin_questions",
        })
}

func adminEditQuestionHandler(w http.ResponseWriter, r *http.Request) {
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        question := getQuestionByID(id)
        if question == nil {
                http.NotFound(w, r)
                return
        }

        if r.Method == "POST" {
                r.ParseMultipartForm(10 << 20)
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                question.Text = r.FormValue("text")
                question.CorrectAnswer = r.FormValue("correct_answer")
                question.VariantsList = parseVariantsFromForm(r)

                file, header, err := r.FormFile("image")
                if err == nil {
                        defer file.Close()
                        imagePath, err := saveUploadedFile(file, header)
                        if err == nil {
                                question.Image = imagePath
                        }
                }

                if r.FormValue("remove_image") == "on" {
                        question.Image = ""
                }

                updateQuestion(question)
                http.Redirect(w, r, "/admin-panel/questions/", http.StatusFound)
                return
        }

        renderTemplate(w, r, "admin/edit_question.html", map[string]interface{}{
                "CurrentPage":  "admin_questions",
                "QuestionData": question,
        })
}

func adminDeleteQuestionHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                id, _ := strconv.Atoi(mux.Vars(r)["id"])
                deleteQuestion(id)
        }
        http.Redirect(w, r, "/admin-panel/questions/", http.StatusFound)
}

func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
        users := getNonStaffUsers()
        renderTemplate(w, r, "admin/users.html", map[string]interface{}{
                "CurrentPage": "admin_users",
                "Users":       users,
        })
}

func adminAddUserHandler(w http.ResponseWriter, r *http.Request) {
        data := map[string]interface{}{
                "CurrentPage": "admin_users",
        }

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                username := r.FormValue("username")
                password := r.FormValue("password")
                if usernameExists(username, 0) {
                        data["Error"] = "Bu login allaqachon mavjud!"
                } else {
                        createUser(username, password, false)
                        http.Redirect(w, r, "/admin-panel/users/", http.StatusFound)
                        return
                }
        }

        renderTemplate(w, r, "admin/add_user.html", data)
}

func adminEditUserHandler(w http.ResponseWriter, r *http.Request) {
        id, _ := strconv.Atoi(mux.Vars(r)["id"])
        editUser := getUserByID(id)
        if editUser == nil || editUser.IsStaff {
                http.NotFound(w, r)
                return
        }

        data := map[string]interface{}{
                "CurrentPage": "admin_users",
                "EditUser":    editUser,
        }

        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                newUsername := r.FormValue("username")
                newPassword := r.FormValue("password")

                if newUsername != editUser.Username && usernameExists(newUsername, editUser.ID) {
                        data["Error"] = "Bu login allaqachon mavjud!"
                } else {
                        updateUserUsername(editUser.ID, newUsername)
                        if newPassword != "" {
                                updateUserPassword(editUser.ID, newPassword)
                        }
                        data["Success"] = "Foydalanuvchi muvaffaqiyatli yangilandi!"
                        editUser = getUserByID(id)
                        data["EditUser"] = editUser
                }
        }

        renderTemplate(w, r, "admin/edit_user.html", data)
}

func adminDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
                r.ParseForm()
                if !verifyCSRFToken(r, w) {
                        http.Error(w, "CSRF token invalid", http.StatusForbidden)
                        return
                }
                id, _ := strconv.Atoi(mux.Vars(r)["id"])
                deleteUser(id)
        }
        http.Redirect(w, r, "/admin-panel/users/", http.StatusFound)
}

func adminStatisticsHandler(w http.ResponseWriter, r *http.Request) {
        users := getNonStaffUsers()
        type UserStat struct {
                User       *User
                TotalTests int
                AvgScore   int
        }
        var userStats []UserStat
        for _, u := range users {
                sessions := getUserCompletedSessions(u.ID)
                total := len(sessions)
                avg := 0
                if total > 0 {
                        sum := 0
                        for _, s := range sessions {
                                sum += s.ScorePercent
                        }
                        avg = sum / total
                }
                userStats = append(userStats, UserStat{
                        User:       u,
                        TotalTests: total,
                        AvgScore:   avg,
                })
        }

        renderTemplate(w, r, "admin/statistics.html", map[string]interface{}{
                "CurrentPage": "admin_statistics",
                "UserStats":   userStats,
        })
}
