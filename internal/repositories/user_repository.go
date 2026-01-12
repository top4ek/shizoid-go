package repositories

// import (
// 	"context"
// 	"database/sql"
// 	"example.com/m/v2/internal/models"
// )
//
// type UserRepository struct {
// 	db *sql.DB
// }
//
// func NewUserRepository(db *sql.DB) *UserRepository {
// 	return &UserRepository{db: db}
// }
//
// func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) (int64, error) {
// 	var id int64
// 	err := r.db.QueryRowContext(ctx, "INSERT INTO users (username) VALUES ($1) RETURNING id", user.Username).Scan(&id)
// 	return id, err
// }
//
// func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
// 	user := &models.User{}
// 	err := r.db.QueryRowContext(ctx, "SELECT id, username, created_at FROM users WHERE id = $1", id).Scan(&user.ID, &user.Username, &user.CreatedAt)
// 	if err == sql.ErrNoRows {
// 		return nil, nil
// 	}
// 	return user, err
// }
