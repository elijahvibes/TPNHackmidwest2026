package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"trailmix/internal/auth"
	"trailmix/internal/httpx"
)

type User struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	HomeCity *string  `json:"home_city"`
	HomeLat  *float64 `json:"home_lat"`
	HomeLng  *float64 `json:"home_lng"`
}

const userCols = "id, email, name, home_city, home_lat, home_lng"

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.HomeCity, &u.HomeLat, &u.HomeLng)
	return u, err
}

func (d *Deps) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(in.Email, "@") || len(in.Password) < 8 || strings.TrimSpace(in.Name) == "" {
		httpx.Error(w, 400, "need a valid email, a name, and a password of 8+ characters")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		httpx.Error(w, 500, "hash failed")
		return
	}
	u, err := scanUser(d.DB.QueryRow(r.Context(),
		"INSERT INTO users (email, name, password_hash) VALUES ($1,$2,$3) RETURNING "+userCols,
		in.Email, strings.TrimSpace(in.Name), hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.Error(w, 409, "email already registered")
			return
		}
		httpx.Error(w, 500, "could not create user")
		return
	}
	d.issue(w, u)
}

func (d *Deps) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	var u User
	var hash string
	err := d.DB.QueryRow(r.Context(),
		"SELECT "+userCols+", password_hash FROM users WHERE email=$1", strings.ToLower(strings.TrimSpace(in.Email))).
		Scan(&u.ID, &u.Email, &u.Name, &u.HomeCity, &u.HomeLat, &u.HomeLng, &hash)
	if err != nil || !auth.CheckPassword(hash, in.Password) {
		httpx.Error(w, 401, "wrong email or password")
		return
	}
	d.issue(w, u)
}

func (d *Deps) issue(w http.ResponseWriter, u User) {
	tok, err := d.Auth.Issue(u.ID, u.Email)
	if err != nil {
		httpx.Error(w, 500, "token failed")
		return
	}
	httpx.JSON(w, 200, map[string]any{"token": tok, "user": u})
}

func (d *Deps) me(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	u, err := scanUser(d.DB.QueryRow(r.Context(), "SELECT "+userCols+" FROM users WHERE id=$1", c.UserID))
	if err != nil {
		httpx.Error(w, 404, "user not found")
		return
	}
	httpx.JSON(w, 200, u)
}

// updateMe stores the browser's geolocation as the user's home base.
func (d *Deps) updateMe(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var in struct {
		HomeCity *string  `json:"home_city"`
		HomeLat  *float64 `json:"home_lat"`
		HomeLng  *float64 `json:"home_lng"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid json")
		return
	}
	u, err := scanUser(d.DB.QueryRow(r.Context(),
		`UPDATE users SET home_city=COALESCE($2,home_city), home_lat=COALESCE($3,home_lat), home_lng=COALESCE($4,home_lng)
		 WHERE id=$1 RETURNING `+userCols, c.UserID, in.HomeCity, in.HomeLat, in.HomeLng))
	if err != nil {
		httpx.Error(w, 500, "update failed")
		return
	}
	httpx.JSON(w, 200, u)
}
