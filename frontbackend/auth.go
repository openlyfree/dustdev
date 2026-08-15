package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "dustdev_session"

type user struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type credentials struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *server) setSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, token, maxAge, "/", s.cfg.CookieDomain, s.cfg.CookieSecure, true)
}

func (s *server) createSession(c *gin.Context, userID string) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	ttl := time.Duration(s.cfg.SessionTTLDays) * 24 * time.Hour
	_, err = s.db.Pool.Exec(c.Request.Context(),
		"INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, now() + $3::interval)",
		token, userID, ttl.String())
	if err != nil {
		return err
	}
	s.setSessionCookie(c, token, int(ttl.Seconds()))
	return nil
}

func (s *server) userFromRequest(c *gin.Context) (user, bool) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil || token == "" {
		return user{}, false
	}
	var u user
	err = s.db.Pool.QueryRow(c.Request.Context(), `
		SELECT u.id, u.email
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()`, token).Scan(&u.ID, &u.Email)
	if err != nil {
		return user{}, false
	}
	return u, true
}

func (s *server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := s.userFromRequest(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Set("user", u)
		c.Next()
	}
}

func (s *server) signup(c *gin.Context) {
	var creds credentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email and a password of 8+ characters required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	var u user
	err = s.db.Pool.QueryRow(c.Request.Context(),
		"INSERT INTO users (email, pass_hash) VALUES ($1, $2) RETURNING id, email",
		creds.Email, string(hash)).Scan(&u.ID, &u.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	if err := s.createSession(c, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (s *server) login(c *gin.Context) {
	var creds credentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}

	var u user
	var passHash string
	err := s.db.Pool.QueryRow(c.Request.Context(),
		"SELECT id, email, pass_hash FROM users WHERE email = $1", creds.Email).
		Scan(&u.ID, &u.Email, &passHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passHash), []byte(creds.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if err := s.createSession(c, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (s *server) logout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookieName); err == nil && token != "" {
		_, _ = s.db.Pool.Exec(c.Request.Context(), "DELETE FROM sessions WHERE token = $1", token)
	}
	s.setSessionCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

func (s *server) me(c *gin.Context) {
	u, _ := c.Get("user")
	c.JSON(http.StatusOK, u)
}
