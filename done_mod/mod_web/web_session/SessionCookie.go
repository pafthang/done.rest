package websession

import (
	"crypto/ecdsa"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SessionCookieID defines the ID of the cookie containing the user sessionID
const SessionCookieID = "session"

// SessionClaims JWT claims in the session cookie
type SessionClaims struct {
	jwt.RegisteredClaims
	// ID: SessionID
	// Audience: RemoteAddress
	// Subject: clientID
	AuthToken string `json:"auth_token"`
	MaxAge    int    `json:"max_age"`
}

// GetSessionCookie retrieves the credentials from the browser cookie.
// If no valid cookie is found then this returns an error.
// Intended for use by middleware. Any updates to the session will not be available in the cookie
// until the next request. In almost all cases use session from context as set by middleware.
func GetSessionCookie(r *http.Request, pubKey *ecdsa.PublicKey) (*SessionClaims, error) {
	cookie, err := r.Cookie(SessionCookieID)
	if err != nil {
		slog.Debug("no session cookie", "remoteAddr", r.RemoteAddr)
		return nil, errors.New("no session cookie")
	}
	// an invalid cookie means what?. A session might still exist so the session
	if cookie.Valid() != nil {
		slog.Info("invalid session cookie", "remoteAddr", r.RemoteAddr)
		return nil, errors.New("invalid session cookie")
	}
	sessionClaims := &SessionClaims{}
	jwtToken, err := jwt.ParseWithClaims(cookie.Value, sessionClaims,
		func(token *jwt.Token) (interface{}, error) {
			return pubKey, nil
		})
	if err != nil {
		// not a valid token
		return sessionClaims, err
	} else if jwtToken == nil || !jwtToken.Valid {
		// would we ever get here?
		return sessionClaims, errors.New("invalid token")
	}
	return sessionClaims, nil
}

// SetSessionCookie when the client has logged in successfully.
// This stores the user login and auth token in a secured 'same-site' cookie.
//
// max-age in seconds, after which the cookie is deleted, use 0 to delete the cookie on browser exit.
//
// The session cookie is restricted to SameSite policy to reduce the risk of CSRF.
// The cookie has the HttpOnly flag set to disable JS access.
// The cookie path has the session prefix
//
// This generates a JWT token, signed by the service key, with claims.
func SetSessionCookie(w http.ResponseWriter,
	sessionID string, clientID string, authToken string, maxAge int, privKey *ecdsa.PrivateKey) error {
	slog.Info("SetSessionCookie", "sessionID", sessionID)
	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:      sessionID,
			Subject: clientID,
			//Issuer: sm.pubKey,
			IssuedAt: &jwt.NumericDate{Time: time.Now()},
		},
		MaxAge:    maxAge,
		AuthToken: authToken,
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	cookieValue, err := jwtToken.SignedString(privKey)
	if err != nil {
		return err
	}
	// TODO: encrypt cookie value

	c := &http.Cookie{
		Name:     SessionCookieID,
		Value:    cookieValue,
		MaxAge:   maxAge,
		HttpOnly: true, // Cookie is not accessible via client-side java (XSS attack)
		//Secure:   true, // TODO: only use with https
		// With SSR, samesite strict should offer good CSRF protection
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}
	http.SetCookie(w, c)
	return err
}

// RemoveSessionCookie removes the cookie. Intended for logout.
func RemoveSessionCookie(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookieID)
	if err != nil {
		slog.Info("No session cookie found", "url", r.URL.String())
		return
	}
	c.Value = ""
	c.MaxAge = -1
	c.Expires = time.Now().Add(-time.Hour)

	http.SetCookie(w, c)
}
