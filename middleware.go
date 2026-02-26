package main

import (
        "log"
        "net/http"
        "runtime/debug"
)

func getCurrentUser(r *http.Request) *User {
        session, err := store.Get(r, "session")
        if err != nil {
                return nil
        }
        userID, ok := session.Values["user_id"].(int)
        if !ok {
                return nil
        }
        return getUserByID(userID)
}

func setCurrentUser(w http.ResponseWriter, r *http.Request, userID int) {
        session, _ := store.Get(r, "session")
        session.Values["user_id"] = userID
        session.Save(r, w)
}

func clearCurrentUser(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        delete(session.Values, "user_id")
        session.Save(r, w)
}

func authRequired(handler http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
                user := getCurrentUser(r)
                if user == nil {
                        http.Redirect(w, r, "/login/", http.StatusFound)
                        return
                }
                handler(w, r)
        }
}

func adminRequired(handler http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
                user := getCurrentUser(r)
                if user == nil {
                        http.Redirect(w, r, "/login/", http.StatusFound)
                        return
                }
                if !user.IsStaff {
                        http.Redirect(w, r, "/dashboard/", http.StatusFound)
                        return
                }
                handler(w, r)
        }
}

func recoveryMiddleware(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                defer func() {
                        if err := recover(); err != nil {
                                log.Printf("PANIC: %v\n%s", err, debug.Stack())
                                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                        }
                }()
                next.ServeHTTP(w, r)
        })
}
