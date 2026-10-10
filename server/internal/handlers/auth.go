package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

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

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.HomeCity, &u.HomeLat, &u.HomeLng)
	return u, err
}

func (d *Deps) userByID(r *http.Request, id string) (User, error) {
	return scanUser(d.DB.QueryRowContext(r.Context(), "SELECT "+userCols+" FROM users WHERE id=?", id))
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
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
	id := uuid.NewString()
	_, err = d.DB.ExecContext(r.Context(),
		"INSERT INTO users (id, email, name, password_hash) VALUES (?,?,?,?)",
		id, in.Email, strings.TrimSpace(in.Name), hash)
	if err != nil {
		if isDuplicate(err) {
			httpx.Error(w, 409, "email already registered")
			return
		}
		httpx.Error(w, 500, "could not create user")
		return
	}
	u, err := d.userByID(r, id)
	if err != nil {
		httpx.Error(w, 500, "could not load user")
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
	err := d.DB.QueryRowContext(r.Context(),
		"SELECT "+userCols+", password_hash FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(in.Email))).
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
	u, err := d.userByID(r, c.UserID)
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
	_, err := d.DB.ExecContext(r.Context(),
		"UPDATE users SET home_city=COALESCE(?,home_city), home_lat=COALESCE(?,home_lat), home_lng=COALESCE(?,home_lng) WHERE id=?",
		in.HomeCity, in.HomeLat, in.HomeLng, c.UserID)
	if err != nil {
		httpx.Error(w, 500, "update failed")
		return
	}
	u, err := d.userByID(r, c.UserID)
	if err != nil {
		httpx.Error(w, 404, "user not found")
		return
	}
	httpx.JSON(w, 200, u)
}
