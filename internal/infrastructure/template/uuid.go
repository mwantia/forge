package template

import "github.com/google/uuid"

func GenerateNewID() string {
	return uuid.New().String()
}
