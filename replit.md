# AvtotestPrime

Auto test question platform built with Go and PostgreSQL.

## Overview
A web application for studying auto test questions with admin panel and user dashboard. Features dark theme throughout, Uzbek language UI, and responsive design.

## Tech Stack
- **Backend**: Go 1.24 (gorilla/mux, gorilla/sessions, lib/pq, bcrypt)
- **Database**: PostgreSQL
- **Frontend**: HTML/CSS/JS (dark theme, responsive)
- **Server**: Go HTTP server on port 5000

## Project Structure
```
main.go              - Entry point, routes, template rendering
db.go                - Database connection, migrations, seed data
models.go            - Data models and database queries
handlers.go          - HTTP request handlers (auth, user, admin)
middleware.go        - Authentication and authorization middleware
go.mod / go.sum      - Go module dependencies
templates/           - Go HTML templates
  admin/             - Admin panel templates
static/
  css/style.css      - Dark theme styles
  js/main.js         - Frontend JavaScript
media/questions/     - Uploaded question images
deploy.sh            - VPS deployment script
update.sh            - VPS update script
diagnose.sh          - VPS diagnostic script
```

## Features

### User Panel
- Login/logout with no password restrictions
- Dashboard with question count, bookmarks, test stats
- Browse all questions with correct answers highlighted
- Search questions by text or number
- Bookmark/save questions
- Random test mode with timer, live score, 1.2s auto-advance
- Statistics tracking
- Profile with username/password change

### Admin Panel
- Dashboard with overview stats and recent tests
- Add/edit/delete questions (2-10 dynamic variants, image upload)
- Manage users (add/edit/delete)
- View user statistics

## Default Users
- Admin: `admin` / `admin`
- User: `user` / `user`

## Database Tables
- **users**: id, username, password_hash, is_staff, date_joined
- **questions**: id, number, text, image, variants_json, correct_answer, variant_a-d, timestamps
- **bookmarks**: user_id + question_id (favorites)
- **test_sessions**: test results with score, question_ids stored as JSON
- **test_answers**: individual answer records

## Running
```
go run .
```
