package httpapi

import (
	"context"

	"github.com/ryankavi/payclone/internal/models"
)

type ctxKey int

const (
	userIDKey ctxKey = iota
	roleKey
)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromCtx(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}

func WithRole(ctx context.Context, role models.UserRole) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

func RoleFromCtx(ctx context.Context) (models.UserRole, bool) {
	role, ok := ctx.Value(roleKey).(models.UserRole)
	return role, ok && role != ""
}
