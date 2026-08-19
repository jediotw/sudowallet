package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"os"
	"time"
)

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// get the JWT secret from the environment variable
func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret-for-local-development" // default secret for local development
	}
	return secret
}

// we have the information what we want to store in the token, we can create a function that will generate a JWT token for us. This function will take in the user ID and email as parameters and return a signed JWT token.
// the method parameter should have when this token will expire set by the application owner so
func GenerateJWT(userID string, email string, duration time.Duration) (string, error) {
	//create the claims
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	//create the token with the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	//sign the token with the secret
	signedToken, err := token.SignedString([]byte(getJWTSecret()))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func ValidateToken(tokenString string) (*JWTClaims, error) {

	// Parse the token.

	// Define a function that returns the key used to verify the token's signature.
	keyFunc := func(token *jwt.Token) (any, error) {

		// Before returning the key, check that the signing method
		// is the one we expect.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenUnverifiable
		}

		return []byte(getJWTSecret()), nil
	}

	// ParseWithClaims takes the token string received from the client,
	// parses it, fills our JWTClaims struct with the claims from the token,
	// and uses keyFunc to obtain the key needed to verify the signature.
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, keyFunc)

	if err != nil {
		return nil, err
	}

	// The parsed claims are stored in token.Claims.
	// Since token.Claims has the interface type jwt.Claims,
	// we use a type assertion to get our concrete *JWTClaims.
	//claims are data supplied by whoever created/signed the JWT, and the client is merely sending the signed JWT back to you.
	// We also check token.Valid to make sure the token passed validation.
	//The type assertion checks whether the token.Claims interface actually contains our expected *JWTClaims type.
	claims, ok := token.Claims.(*JWTClaims)

	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
